package service_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/listings/domain"
	"github.com/trcmkr/hse-business-club/back/internal/listings/service"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

// fakeRepo is an in-memory Repository. closeBeforeWrite simulates a concurrent close
// between the service's read and its conditional write.
type fakeRepo struct {
	mu               sync.Mutex
	items            map[uuid.UUID]*listings.Listing
	clock            time.Time
	closeBeforeWrite bool
	lastFilter       listings.ListFilter
	writes           int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{items: map[uuid.UUID]*listings.Listing{}, clock: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (r *fakeRepo) now() time.Time {
	r.clock = r.clock.Add(time.Minute)
	return r.clock
}

func clone(l *listings.Listing) *listings.Listing {
	c := *l
	c.Tags = append([]string{}, l.Tags...)
	return &c
}

func (r *fakeRepo) Create(_ context.Context, p listings.CreateParams) (*listings.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	l := &listings.Listing{
		ID: uuid.New(), AuthorID: p.AuthorID, Kind: p.Kind, Title: p.Title, Description: p.Description,
		LookingFor: p.LookingFor, Tags: p.Tags, Stage: p.Stage, Compensation: p.Compensation,
		Status: listings.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	r.items[l.ID] = l
	r.writes++
	return clone(l), nil
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*listings.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.items[id]
	if !ok {
		return nil, domain.ErrNotFound()
	}
	return clone(l), nil
}

func (r *fakeRepo) GetMany(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]*listings.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[uuid.UUID]*listings.Listing{}
	for _, id := range ids {
		if l, ok := r.items[id]; ok {
			out[id] = clone(l)
		}
	}
	return out, nil
}

func (r *fakeRepo) sorted(keep func(*listings.Listing) bool) []*listings.Listing {
	out := []*listings.Listing{}
	for _, l := range r.items {
		if keep(l) {
			out = append(out, clone(l))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (r *fakeRepo) ListActive(_ context.Context, f listings.ListFilter) ([]*listings.Listing, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastFilter = f
	all := r.sorted(func(l *listings.Listing) bool {
		return l.Status == listings.StatusActive && (f.Kind == "" || l.Kind == f.Kind)
	})
	from := min(domain.Offset(f), len(all))
	to := min(from+f.Limit, len(all))
	return all[from:to], len(all), nil
}

func (r *fakeRepo) ListByAuthor(_ context.Context, authorID uuid.UUID, status listings.Status) ([]*listings.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sorted(func(l *listings.Listing) bool {
		return l.AuthorID == authorID && (status == "" || l.Status == status)
	}), nil
}

// activeForWrite returns the listing if it can be written, honoring closeBeforeWrite.
func (r *fakeRepo) activeForWrite(id uuid.UUID) (*listings.Listing, bool) {
	l, ok := r.items[id]
	if !ok {
		return nil, false
	}
	if r.closeBeforeWrite {
		r.closeBeforeWrite = false
		now := r.now()
		l.Status, l.ClosedAt = listings.StatusClosed, &now
	}
	return l, l.Status == listings.StatusActive
}

func (r *fakeRepo) Update(_ context.Context, id uuid.UUID, p listings.UpdateParams) (*listings.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.activeForWrite(id)
	if !ok {
		return nil, domain.ErrNotActive
	}
	l.Title, l.Description, l.LookingFor = p.Title, p.Description, p.LookingFor
	l.Tags, l.Stage, l.Compensation = p.Tags, p.Stage, p.Compensation
	l.UpdatedAt = r.now()
	r.writes++
	return clone(l), nil
}

func (r *fakeRepo) Close(_ context.Context, id uuid.UUID) (*listings.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.activeForWrite(id)
	if !ok {
		return nil, domain.ErrNotActive
	}
	now := r.now()
	l.Status, l.ClosedAt, l.UpdatedAt = listings.StatusClosed, &now, now
	r.writes++
	return clone(l), nil
}

func startupParams(author uuid.UUID) listings.CreateParams {
	return listings.CreateParams{
		AuthorID: author, Kind: listings.KindStartup, Title: "  Стартап  ",
		Description: "Описание стартапа", LookingFor: "Разработчик",
		Tags: []string{" Go ", ""}, Stage: listings.StageIdea,
	}
}

func updateParams() listings.UpdateParams {
	return listings.UpdateParams{
		Title: "Новое название", Description: "Новое описание", LookingFor: "Дизайнер",
		Tags: []string{"Figma"}, Stage: listings.StageMVP,
	}
}

func wantErr(t *testing.T, err error, kind apperr.Kind, msg string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind != kind || (msg != "" && ae.Msg != msg) {
		t.Fatalf("err = %v, want kind %v msg %q", err, kind, msg)
	}
}

func setup(t *testing.T) (*service.Service, *fakeRepo, uuid.UUID, *listings.Listing) {
	t.Helper()
	repo := newFakeRepo()
	svc := service.New(repo)
	author := uuid.New()
	l, err := svc.Create(context.Background(), startupParams(author))
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo, author, l
}

func TestCreate(t *testing.T) {
	t.Parallel()
	_, repo, author, l := setup(t)
	if l.Title != "Стартап" || len(l.Tags) != 1 || l.Tags[0] != "Go" || l.AuthorID != author || l.Status != listings.StatusActive {
		t.Fatalf("Create did not normalize: %+v", l)
	}

	svc := service.New(repo)
	p := startupParams(author)
	p.Stage = ""
	_, err := svc.Create(context.Background(), p)
	wantErr(t, err, apperr.KindValidation, "Стадия: укажите стадию")
	if repo.writes != 1 {
		t.Fatalf("invalid create reached the repo: writes = %d", repo.writes)
	}
}

func TestGet(t *testing.T) {
	t.Parallel()
	svc, _, author, l := setup(t)
	ctx := context.Background()
	got, err := svc.Get(ctx, l.ID)
	if err != nil || got.ID != l.ID {
		t.Fatalf("Get = %v, %v", got, err)
	}
	_, err = svc.Get(ctx, uuid.New())
	wantErr(t, err, apperr.KindNotFound, "Публикация не найдена")

	if _, err := svc.Close(ctx, l.ID, author); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Get(ctx, l.ID)
	if err != nil || got.Status != listings.StatusClosed {
		t.Fatalf("Get(closed) = %v, %v", got, err)
	}
}

func TestGetMany(t *testing.T) {
	t.Parallel()
	svc, _, _, l := setup(t)
	got, err := svc.GetMany(context.Background(), []uuid.UUID{l.ID, uuid.New()})
	if err != nil || len(got) != 1 || got[l.ID] == nil {
		t.Fatalf("GetMany = %v, %v", got, err)
	}
	empty, err := svc.GetMany(context.Background(), nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("GetMany(nil) = %v, %v", empty, err)
	}
}

func TestListActive(t *testing.T) {
	t.Parallel()
	svc, repo, author, first := setup(t)
	ctx := context.Background()
	for range 2 {
		if _, err := svc.Create(ctx, startupParams(author)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Close(ctx, first.ID, author); err != nil {
		t.Fatal(err)
	}

	page, err := svc.ListActive(ctx, listings.ListFilter{Query: "  go  "})
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 1 || page.Limit != 20 || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("page = %+v", page)
	}
	if repo.lastFilter.Query != "go" {
		t.Fatalf("repo got query %q, want trimmed", repo.lastFilter.Query)
	}

	page, err = svc.ListActive(ctx, listings.ListFilter{Page: 1000, Limit: 50})
	if err != nil || len(page.Items) != 0 || page.Total != 2 || page.Page != 1000 || page.Limit != 50 {
		t.Fatalf("past the end = %+v, %v", page, err)
	}

	_, err = svc.ListActive(ctx, listings.ListFilter{Limit: 51})
	wantErr(t, err, apperr.KindValidation, "Размер страницы: от 1 до 50")
	_, err = svc.ListActive(ctx, listings.ListFilter{Page: -1})
	wantErr(t, err, apperr.KindValidation, "Страница: не меньше 1")
}

func TestListByAuthor(t *testing.T) {
	t.Parallel()
	svc, _, author, l := setup(t)
	ctx := context.Background()
	if _, err := svc.Close(ctx, l.ID, author); err != nil {
		t.Fatal(err)
	}
	all, err := svc.ListByAuthor(ctx, author, "")
	if err != nil || len(all) != 1 {
		t.Fatalf("ListByAuthor = %v, %v", all, err)
	}
	active, err := svc.ListByAuthor(ctx, author, listings.StatusActive)
	if err != nil || len(active) != 0 {
		t.Fatalf("ListByAuthor(active) = %v, %v", active, err)
	}
	_, err = svc.ListByAuthor(ctx, author, "draft")
	wantErr(t, err, apperr.KindValidation, "")
}

func TestUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("author edits active", func(t *testing.T) {
		t.Parallel()
		svc, _, author, l := setup(t)
		got, err := svc.Update(ctx, l.ID, author, updateParams())
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "Новое название" || got.Stage != listings.StageMVP || got.Kind != listings.KindStartup {
			t.Fatalf("Update = %+v", got)
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		svc, _, author, _ := setup(t)
		_, err := svc.Update(ctx, uuid.New(), author, updateParams())
		wantErr(t, err, apperr.KindNotFound, "Публикация не найдена")
	})
	t.Run("non-author forbidden", func(t *testing.T) {
		t.Parallel()
		svc, repo, _, l := setup(t)
		_, err := svc.Update(ctx, l.ID, uuid.New(), updateParams())
		wantErr(t, err, apperr.KindForbidden, "Недостаточно прав")
		if repo.writes != 1 {
			t.Fatal("forbidden update reached the repo")
		}
	})
	t.Run("non-author on closed is forbidden, not precondition", func(t *testing.T) {
		t.Parallel()
		svc, _, author, l := setup(t)
		if _, err := svc.Close(ctx, l.ID, author); err != nil {
			t.Fatal(err)
		}
		_, err := svc.Update(ctx, l.ID, uuid.New(), updateParams())
		wantErr(t, err, apperr.KindForbidden, "Недостаточно прав")
	})
	t.Run("closed", func(t *testing.T) {
		t.Parallel()
		svc, _, author, l := setup(t)
		if _, err := svc.Close(ctx, l.ID, author); err != nil {
			t.Fatal(err)
		}
		_, err := svc.Update(ctx, l.ID, author, updateParams())
		wantErr(t, err, apperr.KindPrecondition, "Публикация закрыта")
	})
	t.Run("validation uses stored kind", func(t *testing.T) {
		t.Parallel()
		svc, _, author, l := setup(t)
		p := updateParams()
		p.Stage = ""
		p.Compensation = "Оплата"
		_, err := svc.Update(ctx, l.ID, author, p)
		wantErr(t, err, apperr.KindValidation, "Стадия: укажите стадию")
	})
	t.Run("closed concurrently", func(t *testing.T) {
		t.Parallel()
		svc, repo, author, l := setup(t)
		repo.closeBeforeWrite = true
		_, err := svc.Update(ctx, l.ID, author, updateParams())
		wantErr(t, err, apperr.KindPrecondition, "Публикация закрыта")
	})
}

func TestClose(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("author closes, then idempotent", func(t *testing.T) {
		t.Parallel()
		svc, repo, author, l := setup(t)
		closed, err := svc.Close(ctx, l.ID, author)
		if err != nil {
			t.Fatal(err)
		}
		if closed.Status != listings.StatusClosed || closed.ClosedAt == nil {
			t.Fatalf("Close = %+v", closed)
		}
		writes := repo.writes
		again, err := svc.Close(ctx, l.ID, author)
		if err != nil {
			t.Fatal(err)
		}
		if again.Status != listings.StatusClosed || !again.ClosedAt.Equal(*closed.ClosedAt) {
			t.Fatalf("second Close = %+v", again)
		}
		if repo.writes != writes {
			t.Fatal("repeated close wrote to the repo")
		}
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		svc, _, author, _ := setup(t)
		_, err := svc.Close(ctx, uuid.New(), author)
		wantErr(t, err, apperr.KindNotFound, "Публикация не найдена")
	})
	t.Run("non-author forbidden", func(t *testing.T) {
		t.Parallel()
		svc, _, _, l := setup(t)
		_, err := svc.Close(ctx, l.ID, uuid.New())
		wantErr(t, err, apperr.KindForbidden, "Недостаточно прав")
	})
	t.Run("non-author on closed forbidden", func(t *testing.T) {
		t.Parallel()
		svc, _, author, l := setup(t)
		if _, err := svc.Close(ctx, l.ID, author); err != nil {
			t.Fatal(err)
		}
		_, err := svc.Close(ctx, l.ID, uuid.New())
		wantErr(t, err, apperr.KindForbidden, "Недостаточно прав")
	})
	t.Run("closed concurrently", func(t *testing.T) {
		t.Parallel()
		svc, repo, author, l := setup(t)
		repo.closeBeforeWrite = true
		got, err := svc.Close(ctx, l.ID, author)
		if err != nil || got.Status != listings.StatusClosed {
			t.Fatalf("Close = %v, %v", got, err)
		}
	})
}
