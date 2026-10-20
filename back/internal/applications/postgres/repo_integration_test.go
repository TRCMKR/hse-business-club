//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/applications/domain"
	"github.com/trcmkr/hse-business-club/back/internal/applications/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db/dbtest"
)

func setup(t *testing.T) (*postgres.Repo, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Migrate(t, pool)
	return postgres.New(pool), pool
}

func create(t *testing.T, r *postgres.Repo, listing, author, applicant uuid.UUID) *applications.Application {
	t.Helper()
	a, err := r.Create(context.Background(), &applications.Application{
		ListingID: listing, ListingAuthorID: author, ApplicantID: applicant, Message: "hi",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return a
}

func setStatus(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, s applications.Status) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE applications.applications SET status = $2 WHERE id = $1`, id, string(s)); err != nil {
		t.Fatal(err)
	}
}

func setCreatedAt(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, at time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE applications.applications SET created_at = $2 WHERE id = $1`, id, at); err != nil {
		t.Fatal(err)
	}
}

func ids(list []*applications.Application) []uuid.UUID {
	out := make([]uuid.UUID, len(list))
	for i, a := range list {
		out[i] = a.ID
	}
	return out
}

func equalIDs(t *testing.T, got []*applications.Application, want ...uuid.UUID) {
	t.Helper()
	g := ids(got)
	if len(g) != len(want) {
		t.Fatalf("got %d items %v, want %d %v", len(g), g, len(want), want)
	}
	for i, w := range want {
		if g[i] != w {
			t.Fatalf("item %d = %v, want %v (got %v)", i, g[i], w, g)
		}
	}
}

func TestMigrations(t *testing.T) {
	_, pool := setup(t)
	ctx := context.Background()
	if err := dbtest.Up(ctx, pool); err != nil {
		t.Fatalf("second Up: %v", err)
	}
	var ok bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('applications.applications') IS NOT NULL
		AND to_regclass('applications.applications_listing_author_idx') IS NOT NULL
		AND to_regclass('public.goose_db_version') IS NOT NULL`).Scan(&ok); err != nil || !ok {
		t.Fatalf("schema objects missing: ok=%v err=%v", ok, err)
	}
}

func TestCreateGet(t *testing.T) {
	r, _ := setup(t)
	ctx := context.Background()
	listing, author, applicant := uuid.New(), uuid.New(), uuid.New()

	a := create(t, r, listing, author, applicant)
	if a.ID == uuid.Nil || a.Status != applications.StatusPending || a.DecidedAt != nil ||
		a.ListingID != listing || a.ListingAuthorID != author || a.ApplicantID != applicant ||
		a.Message != "hi" || a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() {
		t.Fatalf("created = %+v", a)
	}
	got, err := r.GetByID(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if *got != *a {
		t.Fatalf("get = %+v, want %+v", got, a)
	}

	_, err = r.GetByID(ctx, uuid.New())
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("missing: err = %v", err)
	}
}

func TestUniqueLiveApplication(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	listing, author, applicant := uuid.New(), uuid.New(), uuid.New()

	first := create(t, r, listing, author, applicant)
	conflict := func(when string) {
		t.Helper()
		_, err := r.Create(ctx, &applications.Application{ListingID: listing, ListingAuthorID: author, ApplicantID: applicant, Message: "again"})
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind != apperr.KindConflict || ae.Msg != "Вы уже откликались на эту публикацию" {
			t.Fatalf("%s: err = %v, want conflict", when, err)
		}
	}
	conflict("pending")

	// Other applicant / other listing are independent.
	create(t, r, listing, author, uuid.New())
	create(t, r, uuid.New(), author, applicant)

	for _, s := range []applications.Status{applications.StatusAccepted, applications.StatusDeclined} {
		setStatus(t, pool, first.ID, s)
		conflict(string(s))
	}

	setStatus(t, pool, first.ID, applications.StatusWithdrawn)
	second := create(t, r, listing, author, applicant)
	if second.ID == first.ID {
		t.Fatal("re-apply must create a new record")
	}
	conflict("pending after re-apply")
}

func TestCreateRace(t *testing.T) {
	r, _ := setup(t)
	listing, author, applicant := uuid.New(), uuid.New(), uuid.New()

	const n = 8
	var (
		wg               sync.WaitGroup
		mu               sync.Mutex
		okCnt, conflicts int
		others           []error
	)
	start := make(chan struct{})
	for range n {
		wg.Go(func() {
			<-start
			_, err := r.Create(context.Background(), &applications.Application{
				ListingID: listing, ListingAuthorID: author, ApplicantID: applicant, Message: "race",
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				okCnt++
			case apperr.KindOf(err) == apperr.KindConflict:
				conflicts++
			default:
				others = append(others, err)
			}
		})
	}
	close(start)
	wg.Wait()
	if okCnt != 1 || conflicts != n-1 || len(others) != 0 {
		t.Fatalf("ok=%d conflicts=%d others=%v", okCnt, conflicts, others)
	}
}

func TestUpdate(t *testing.T) {
	r, _ := setup(t)
	ctx := context.Background()
	a := create(t, r, uuid.New(), uuid.New(), uuid.New())

	// nil change → no write, current state returned.
	got, err := r.Update(ctx, a.ID, func(*applications.Application) (*domain.StatusChange, error) {
		return nil, nil
	})
	if err != nil || *got != *a {
		t.Fatalf("noop: got %+v err %v", got, err)
	}

	// fn error aborts.
	boom := errors.New("boom")
	if _, err := r.Update(ctx, a.ID, func(*applications.Application) (*domain.StatusChange, error) {
		return nil, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}

	decided := time.Now().UTC().Truncate(time.Microsecond)
	got, err = r.Update(ctx, a.ID, func(cur *applications.Application) (*domain.StatusChange, error) {
		if cur.Status != applications.StatusPending {
			t.Errorf("cur status = %s", cur.Status)
		}
		return &domain.StatusChange{Status: applications.StatusAccepted, DecidedAt: &decided}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != applications.StatusAccepted || got.DecidedAt == nil || !got.DecidedAt.Equal(decided) ||
		got.UpdatedAt.Before(a.UpdatedAt) {
		t.Fatalf("updated = %+v", got)
	}
	reread, err := r.GetByID(ctx, a.ID)
	if err != nil || reread.Status != applications.StatusAccepted {
		t.Fatalf("reread = %+v, %v", reread, err)
	}

	if _, err := r.Update(ctx, uuid.New(), func(*applications.Application) (*domain.StatusChange, error) {
		t.Error("fn must not be called for a missing row")
		return nil, nil
	}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("missing: err = %v", err)
	}
}

func TestListSorting(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	author := uuid.New()
	listing, otherListing := uuid.New(), uuid.New()
	applicant := uuid.New()
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	// listing: a1 (oldest, accepted), a2 (pending), a3 (declined, newest), a4 (pending, newest pending)
	a1 := create(t, r, listing, author, uuid.New())
	a2 := create(t, r, listing, author, applicant)
	a3 := create(t, r, listing, author, uuid.New())
	a4 := create(t, r, listing, author, uuid.New())
	// another listing of the same author, sent by the same applicant
	b1 := create(t, r, otherListing, author, applicant)
	// foreign listing, sent by the same applicant
	c1 := create(t, r, uuid.New(), uuid.New(), applicant)

	setCreatedAt(t, pool, a1.ID, base.Add(1*time.Hour))
	setCreatedAt(t, pool, a2.ID, base.Add(2*time.Hour))
	setCreatedAt(t, pool, a3.ID, base.Add(6*time.Hour))
	setCreatedAt(t, pool, a4.ID, base.Add(4*time.Hour))
	setCreatedAt(t, pool, b1.ID, base.Add(5*time.Hour))
	setCreatedAt(t, pool, c1.ID, base.Add(3*time.Hour))
	setStatus(t, pool, a1.ID, applications.StatusAccepted)
	setStatus(t, pool, a3.ID, applications.StatusDeclined)

	got, err := r.ListByListing(ctx, listing, "")
	if err != nil {
		t.Fatal(err)
	}
	equalIDs(t, got, a4.ID, a2.ID, a3.ID, a1.ID)

	got, err = r.ListByListing(ctx, listing, applications.StatusPending)
	if err != nil {
		t.Fatal(err)
	}
	equalIDs(t, got, a4.ID, a2.ID)

	got, err = r.ListByListing(ctx, uuid.New(), "")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty list: %v %v", got, err)
	}

	got, err = r.ListByListingAuthor(ctx, author, "")
	if err != nil {
		t.Fatal(err)
	}
	equalIDs(t, got, b1.ID, a4.ID, a2.ID, a3.ID, a1.ID)

	got, err = r.ListByListingAuthor(ctx, author, applications.StatusDeclined)
	if err != nil {
		t.Fatal(err)
	}
	equalIDs(t, got, a3.ID)

	// Sent: newest first regardless of status.
	setStatus(t, pool, b1.ID, applications.StatusAccepted)
	got, err = r.ListByApplicant(ctx, applicant, "")
	if err != nil {
		t.Fatal(err)
	}
	equalIDs(t, got, b1.ID, c1.ID, a2.ID)

	got, err = r.ListByApplicant(ctx, applicant, applications.StatusPending)
	if err != nil {
		t.Fatal(err)
	}
	equalIDs(t, got, c1.ID, a2.ID)
}

func TestGetMineForListings(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	me, author := uuid.New(), uuid.New()
	l1, l2, l3, l4 := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	live1 := create(t, r, l1, author, me)
	old2 := create(t, r, l2, author, me)
	setStatus(t, pool, old2.ID, applications.StatusWithdrawn)
	live2 := create(t, r, l2, author, me) // re-applied after withdraw
	w3 := create(t, r, l3, author, me)
	setStatus(t, pool, w3.ID, applications.StatusWithdrawn) // only a withdrawn one
	create(t, r, l4, author, uuid.New())                    // somebody else's
	d4 := create(t, r, uuid.New(), author, me)              // not requested
	setStatus(t, pool, d4.ID, applications.StatusDeclined)

	got, err := r.GetMineForListings(ctx, me, []uuid.UUID{l1, l2, l3, l4})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[l1] == nil || got[l1].ID != live1.ID || got[l2] == nil || got[l2].ID != live2.ID {
		t.Fatalf("got %v", got)
	}

	// Declined counts as live.
	setStatus(t, pool, live1.ID, applications.StatusDeclined)
	got, err = r.GetMineForListings(ctx, me, []uuid.UUID{l1})
	if err != nil || got[l1] == nil || got[l1].Status != applications.StatusDeclined {
		t.Fatalf("declined: %v %v", got, err)
	}

	got, err = r.GetMineForListings(ctx, me, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty ids: %v %v", got, err)
	}
}

func TestStatsForListings(t *testing.T) {
	r, pool := setup(t)
	ctx := context.Background()
	author := uuid.New()
	l1, l2, empty := uuid.New(), uuid.New(), uuid.New()

	for _, s := range []applications.Status{
		applications.StatusPending, applications.StatusPending, applications.StatusAccepted,
		applications.StatusDeclined, applications.StatusWithdrawn, applications.StatusWithdrawn,
	} {
		a := create(t, r, l1, author, uuid.New())
		setStatus(t, pool, a.ID, s)
	}
	a := create(t, r, l2, author, uuid.New())
	setStatus(t, pool, a.ID, applications.StatusWithdrawn)
	create(t, r, uuid.New(), author, uuid.New()) // not requested

	got, err := r.StatsForListings(ctx, []uuid.UUID{l1, l2, empty})
	if err != nil {
		t.Fatal(err)
	}
	want := map[uuid.UUID]applications.Stats{
		l1:    {Pending: 2, Accepted: 1, Declined: 1, Total: 4},
		l2:    {},
		empty: {},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for id, w := range want {
		g, ok := got[id]
		if !ok || g != w {
			t.Fatalf("stats[%v] = %+v (present=%v), want %+v", id, g, ok, w)
		}
	}

	got, err = r.StatsForListings(ctx, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty ids: %v %v", got, err)
	}
}
