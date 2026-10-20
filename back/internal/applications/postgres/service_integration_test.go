//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/applications/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/applications/service"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db/dbtest"
)

// realEnv wires the real service to the real repository; DB tests live in this package
// only, because they share the "applications" schema and must not run in parallel.
func realEnv(t *testing.T) (*service.Service, *stubListings, uuid.UUID, uuid.UUID, func() int) {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Migrate(t, pool)
	author, listing := uuid.New(), uuid.New()
	listings := &stubListings{items: map[uuid.UUID]applications.ListingInfo{
		listing: {ID: listing, AuthorID: author, IsOpen: true},
	}}
	count := func() int {
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM applications.applications`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	return service.New(postgres.New(pool), listings), listings, author, listing, count
}

// runConcurrently starts n calls at once and returns their errors.
func runConcurrently(n int, call func() error) []error {
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Go(func() {
			<-start
			errs[i] = call()
		})
	}
	close(start)
	wg.Wait()
	return errs
}

func TestApplyRace(t *testing.T) {
	svc, _, _, listing, count := realEnv(t)
	me := uuid.New()

	errs := runConcurrently(2, func() error {
		_, err := svc.Apply(context.Background(), listing, me, "гонка")
		return err
	})
	var ok, conflict int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case apperr.KindOf(err) == apperr.KindConflict:
			conflict++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("ok=%d conflict=%d, want 1 and 1", ok, conflict)
	}
	if n := count(); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}

func TestDecisionRace(t *testing.T) {
	svc, _, author, listing, _ := realEnv(t)
	ctx := context.Background()
	me := uuid.New()
	a, err := svc.Apply(ctx, listing, me, "кто первый")
	if err != nil {
		t.Fatal(err)
	}

	// accept, decline and withdraw at once: exactly one wins, the rest see the final status.
	calls := []func() (*applications.Application, error){
		func() (*applications.Application, error) { return svc.Accept(ctx, a.ID, author) },
		func() (*applications.Application, error) { return svc.Decline(ctx, a.ID, author) },
		func() (*applications.Application, error) { return svc.Withdraw(ctx, a.ID, me) },
	}
	results := make([]*applications.Application, len(calls))
	errs := make([]error, len(calls))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i, call := range calls {
		wg.Go(func() {
			<-start
			results[i], errs[i] = call()
		})
	}
	close(start)
	wg.Wait()

	var winners int
	for i, err := range errs {
		switch {
		case err == nil:
			winners++
		case apperr.KindOf(err) == apperr.KindPrecondition:
		default:
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if winners != 1 {
		t.Fatalf("winners = %d, errs = %v", winners, errs)
	}
	final, err := svc.Get(ctx, a.ID, me)
	if err != nil || final.Status == applications.StatusPending {
		t.Fatalf("final = %+v %v", final, err)
	}
}

func TestFlowOnRealDB(t *testing.T) {
	svc, _, author, listing, count := realEnv(t)
	ctx := context.Background()
	me := uuid.New()

	a, err := svc.Apply(ctx, listing, me, "первый")
	if err != nil {
		t.Fatal(err)
	}
	if a.ListingAuthorID != author {
		t.Fatalf("listing_author_id = %v, want %v", a.ListingAuthorID, author)
	}
	if _, err := svc.Withdraw(ctx, a.ID, me); err != nil {
		t.Fatal(err)
	}
	b, err := svc.Apply(ctx, listing, me, "второй")
	if err != nil {
		t.Fatalf("re-apply after withdraw: %v", err)
	}
	acc, err := svc.Accept(ctx, b.ID, author)
	if err != nil || acc.DecidedAt == nil {
		t.Fatalf("accept: %+v %v", acc, err)
	}
	again, err := svc.Accept(ctx, b.ID, author)
	if err != nil || !again.UpdatedAt.Equal(acc.UpdatedAt) || !again.DecidedAt.Equal(*acc.DecidedAt) {
		t.Fatalf("repeat accept must be a no-op: %+v %v", again, err)
	}
	if _, err := svc.Apply(ctx, listing, me, "третий"); apperr.KindOf(err) != apperr.KindConflict {
		t.Fatalf("apply while accepted: %v", err)
	}

	list, err := svc.ListByListing(ctx, listing, author, "")
	if err != nil || len(list) != 2 || list[0].ID != b.ID {
		t.Fatalf("list = %v %v", list, err)
	}
	st, err := svc.StatsForListings(ctx, []uuid.UUID{listing})
	if err != nil || st[listing] != (applications.Stats{Accepted: 1, Total: 1}) {
		t.Fatalf("stats = %v %v", st, err)
	}
	if n := count(); n != 2 {
		t.Fatalf("rows = %d", n)
	}
}

// stubListings implements applications.ListingReader for the service-on-real-DB tests.
type stubListings struct {
	items map[uuid.UUID]applications.ListingInfo
}

func (s *stubListings) GetListing(_ context.Context, id uuid.UUID) (applications.ListingInfo, error) {
	l, ok := s.items[id]
	if !ok {
		return applications.ListingInfo{}, apperr.New(apperr.KindNotFound, "Публикация не найдена")
	}
	return l, nil
}
