//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/listings/domain"
	"github.com/trcmkr/hse-business-club/back/internal/listings/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db/dbtest"
)

// Tests share the "listings" schema, so they run sequentially.

func setup(t *testing.T) (*postgres.Repository, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Migrate(t, pool, postgres.Migrator)
	return postgres.New(pool), pool
}

func params(author uuid.UUID, kind listings.Kind, title, description string) listings.CreateParams {
	p := listings.CreateParams{
		AuthorID: author, Kind: kind, Title: title, Description: description,
		LookingFor: "Кого-нибудь", Tags: []string{"Go", "SQL"},
	}
	if kind == listings.KindStartup {
		p.Stage = listings.StageIdea
	} else {
		p.Compensation = "Доля"
	}
	return p
}

// create inserts a listing and pins its created_at to base+minutes for deterministic order.
func create(t *testing.T, r *postgres.Repository, pool *pgxpool.Pool, p listings.CreateParams, minutes int) *listings.Listing {
	t.Helper()
	ctx := context.Background()
	l, err := r.Create(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute)
	if _, err := pool.Exec(ctx, `UPDATE listings.listings SET created_at = $2 WHERE id = $1`, l.ID, at); err != nil {
		t.Fatal(err)
	}
	l.CreatedAt = at
	return l
}

func ids(items []*listings.Listing) []uuid.UUID {
	out := make([]uuid.UUID, len(items))
	for i, l := range items {
		out[i] = l.ID
	}
	return out
}

func TestMigratorIdempotent(t *testing.T) {
	_, pool := setup(t)
	if err := postgres.Migrator.Up(context.Background(), pool); err != nil {
		t.Fatalf("second Up: %v", err)
	}
}

func TestCreateGet(t *testing.T) {
	r, _ := setup(t)
	ctx := context.Background()
	author := uuid.New()

	s, err := r.Create(ctx, params(author, listings.KindStartup, "Стартап", "Описание стартапа"))
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == uuid.Nil || s.Status != listings.StatusActive || s.Stage != listings.StageIdea ||
		s.Compensation != "" || s.ClosedAt != nil || s.CreatedAt.IsZero() || !reflect.DeepEqual(s.Tags, []string{"Go", "SQL"}) {
		t.Fatalf("created startup: %+v", s)
	}
	got, err := r.GetByID(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, s) {
		t.Fatalf("GetByID = %+v, want %+v", got, s)
	}

	vp := params(author, listings.KindVacancy, "Вакансия", "Описание вакансии")
	vp.Tags = nil
	v, err := r.Create(ctx, vp)
	if err != nil {
		t.Fatal(err)
	}
	if v.Stage != "" || v.Compensation != "Доля" || v.Tags == nil || len(v.Tags) != 0 {
		t.Fatalf("created vacancy: %+v", v)
	}

	_, err = r.GetByID(ctx, uuid.New())
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("GetByID(missing) = %v, want not found", err)
	}
}

func TestGetMany(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	a := create(t, r, pool, params(uuid.New(), listings.KindStartup, "A стартап", "Описание A A A"), 1)
	b := create(t, r, pool, params(uuid.New(), listings.KindVacancy, "B вакансия", "Описание B B B"), 2)
	if _, err := r.Close(ctx, b.ID); err != nil {
		t.Fatal(err)
	}

	got, err := r.GetMany(ctx, []uuid.UUID{a.ID, b.ID, uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[a.ID] == nil || got[b.ID] == nil || got[b.ID].Status != listings.StatusClosed {
		t.Fatalf("GetMany = %v", got)
	}
	empty, err := r.GetMany(ctx, nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("GetMany(nil) = %v, %v", empty, err)
	}
}

func TestListActive(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	author := uuid.New()

	// 5 startups and 4 vacancies, created in order; one of each is closed.
	var startups, vacancies []*listings.Listing
	for i := range 5 {
		startups = append(startups, create(t, r, pool,
			params(author, listings.KindStartup, fmt.Sprintf("Стартап %d", i), "Описание стартапа"), i*2))
	}
	for i := range 4 {
		vacancies = append(vacancies, create(t, r, pool,
			params(author, listings.KindVacancy, fmt.Sprintf("Вакансия %d", i), "Описание вакансии"), i*2+1))
	}
	for _, l := range []*listings.Listing{startups[4], vacancies[0]} {
		if _, err := r.Close(ctx, l.ID); err != nil {
			t.Fatal(err)
		}
	}
	// Active, newest first: v3(7) s3(6) v2(5) s2(4) v1(3) s1(2) s0(0).
	wantAll := []uuid.UUID{vacancies[3].ID, startups[3].ID, vacancies[2].ID, startups[2].ID,
		vacancies[1].ID, startups[1].ID, startups[0].ID}

	tests := []struct {
		name  string
		f     listings.ListFilter
		want  []uuid.UUID
		total int
	}{
		{"all", listings.ListFilter{Page: 1, Limit: 20}, wantAll, 7},
		{"page 1", listings.ListFilter{Page: 1, Limit: 3}, wantAll[:3], 7},
		{"page 2", listings.ListFilter{Page: 2, Limit: 3}, wantAll[3:6], 7},
		{"page 3", listings.ListFilter{Page: 3, Limit: 3}, wantAll[6:], 7},
		{"past the end", listings.ListFilter{Page: 1000, Limit: 50}, []uuid.UUID{}, 7},
		{"startups", listings.ListFilter{Kind: listings.KindStartup, Page: 1, Limit: 20},
			[]uuid.UUID{startups[3].ID, startups[2].ID, startups[1].ID, startups[0].ID}, 4},
		{"vacancies", listings.ListFilter{Kind: listings.KindVacancy, Page: 1, Limit: 2},
			[]uuid.UUID{vacancies[3].ID, vacancies[2].ID}, 3},
		{"query title", listings.ListFilter{Query: "стартап 2", Page: 1, Limit: 20}, []uuid.UUID{startups[2].ID}, 1},
		{"query case-insensitive", listings.ListFilter{Query: "ВАКАНСИЯ", Page: 1, Limit: 20},
			[]uuid.UUID{vacancies[3].ID, vacancies[2].ID, vacancies[1].ID}, 3},
		{"query closed only", listings.ListFilter{Query: "Стартап 4", Page: 1, Limit: 20}, []uuid.UUID{}, 0},
		{"query kind mismatch", listings.ListFilter{Kind: listings.KindVacancy, Query: "Стартап", Page: 1, Limit: 20}, []uuid.UUID{}, 0},
		{"percent matches nothing", listings.ListFilter{Query: "%", Page: 1, Limit: 20}, []uuid.UUID{}, 0},
		{"underscore matches nothing", listings.ListFilter{Query: "_", Page: 1, Limit: 20}, []uuid.UUID{}, 0},
		{"backslash matches nothing", listings.ListFilter{Query: `\`, Page: 1, Limit: 20}, []uuid.UUID{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := r.ListActive(ctx, tt.f)
			if err != nil {
				t.Fatal(err)
			}
			if total != tt.total {
				t.Errorf("total = %d, want %d", total, tt.total)
			}
			if items == nil {
				t.Fatal("items must not be nil")
			}
			if got := ids(items); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ids = %v, want %v", got, tt.want)
			}
			for _, l := range items {
				if l.Status != listings.StatusActive {
					t.Errorf("closed listing %s in feed", l.ID)
				}
			}
		})
	}
}

func TestListActiveSearchMetacharacters(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	author := uuid.New()
	pct := create(t, r, pool, params(author, listings.KindVacancy, "Доля 10% в проекте", "Описание вакансии"), 1)
	und := create(t, r, pool, params(author, listings.KindVacancy, "Вакансия", "Пишем на snake_case"), 2)
	bsl := create(t, r, pool, params(author, listings.KindVacancy, `Путь C:\dev`, "Описание вакансии"), 3)
	create(t, r, pool, params(author, listings.KindVacancy, "Доля 100 в проекте", "snakeXcase и прочее"), 4)

	tests := []struct {
		query string
		want  []uuid.UUID
	}{
		{"%", []uuid.UUID{pct.ID}},
		{"10%", []uuid.UUID{pct.ID}},
		{"_", []uuid.UUID{und.ID}},
		{"snake_case", []uuid.UUID{und.ID}},
		{`\`, []uuid.UUID{bsl.ID}},
		{`C:\dev`, []uuid.UUID{bsl.ID}},
		{"%%", []uuid.UUID{}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			items, total, err := r.ListActive(ctx, listings.ListFilter{Query: tt.query, Page: 1, Limit: 20})
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(items); !reflect.DeepEqual(got, tt.want) || total != len(tt.want) {
				t.Fatalf("query %q: ids = %v total = %d, want %v", tt.query, got, total, tt.want)
			}
		})
	}
}

func TestListByAuthor(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	author, other := uuid.New(), uuid.New()
	a1 := create(t, r, pool, params(author, listings.KindStartup, "Первый", "Описание первого"), 1)
	a2 := create(t, r, pool, params(author, listings.KindVacancy, "Второй", "Описание второго"), 2)
	a3 := create(t, r, pool, params(author, listings.KindVacancy, "Третий", "Описание третьего"), 3)
	create(t, r, pool, params(other, listings.KindVacancy, "Чужой", "Описание чужого"), 4)
	if _, err := r.Close(ctx, a2.ID); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		status listings.Status
		want   []uuid.UUID
	}{
		{"", []uuid.UUID{a3.ID, a2.ID, a1.ID}},
		{listings.StatusActive, []uuid.UUID{a3.ID, a1.ID}},
		{listings.StatusClosed, []uuid.UUID{a2.ID}},
	}
	for _, tt := range tests {
		items, err := r.ListByAuthor(ctx, author, tt.status)
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(items); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("status %q: ids = %v, want %v", tt.status, got, tt.want)
		}
	}
	none, err := r.ListByAuthor(ctx, uuid.New(), "")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("ListByAuthor(nobody) = %v, %v", none, err)
	}
}

func TestUpdate(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	l := create(t, r, pool, params(uuid.New(), listings.KindStartup, "Старое", "Старое описание"), 1)

	up := updParams()
	got, err := r.Update(ctx, l.ID, up)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Новое" || got.Description != "Новое описание" || got.LookingFor != "Дизайнер" ||
		!reflect.DeepEqual(got.Tags, []string{"Figma"}) || got.Stage != listings.StageLaunched ||
		got.Kind != listings.KindStartup || got.AuthorID != l.AuthorID || !got.CreatedAt.Equal(l.CreatedAt) ||
		!got.UpdatedAt.After(l.UpdatedAt) {
		t.Fatalf("Update = %+v", got)
	}

	if _, err := r.Update(ctx, uuid.New(), up); !errors.Is(err, domain.ErrNotActive) {
		t.Fatalf("Update(missing) = %v", err)
	}
	if _, err := r.Close(ctx, l.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Update(ctx, l.ID, up); !errors.Is(err, domain.ErrNotActive) {
		t.Fatalf("Update(closed) = %v", err)
	}
}

func TestClose(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	l := create(t, r, pool, params(uuid.New(), listings.KindVacancy, "Закрыть", "Описание вакансии"), 1)

	closed, err := r.Close(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Status != listings.StatusClosed || closed.ClosedAt == nil {
		t.Fatalf("Close = %+v", closed)
	}
	if _, err := r.Close(ctx, l.ID); !errors.Is(err, domain.ErrNotActive) {
		t.Fatalf("second Close = %v", err)
	}
	again, err := r.GetByID(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !again.ClosedAt.Equal(*closed.ClosedAt) {
		t.Fatal("closed_at changed on repeated close")
	}
	if _, err := r.Close(ctx, uuid.New()); !errors.Is(err, domain.ErrNotActive) {
		t.Fatalf("Close(missing) = %v", err)
	}
}

func updParams() listings.UpdateParams {
	return listings.UpdateParams{
		Title: "Новое", Description: "Новое описание", LookingFor: "Дизайнер",
		Tags: []string{"Figma"}, Stage: listings.StageLaunched,
	}
}
