package service_test

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/applications/domain"
	"github.com/trcmkr/hse-business-club/back/internal/applications/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/applications/service"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

var _ service.Repository = (*postgres.Repo)(nil)

// fakeListings implements applications.ListingReader.
type fakeListings struct {
	items map[uuid.UUID]applications.ListingInfo
	calls int
}

func (f *fakeListings) GetListing(_ context.Context, id uuid.UUID) (applications.ListingInfo, error) {
	f.calls++
	l, ok := f.items[id]
	if !ok {
		return applications.ListingInfo{}, apperr.New(apperr.KindNotFound, "Публикация не найдена")
	}
	return l, nil
}

// fakeRepo is an in-memory service.Repository with the same semantics as postgres.Repo.
type fakeRepo struct {
	mu      sync.Mutex
	items   map[uuid.UUID]*applications.Application
	clock   time.Time
	writes  int // Create + status updates
	listArg applications.Status
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{items: map[uuid.UUID]*applications.Application{}, clock: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
}

func clone(a *applications.Application) *applications.Application {
	c := *a
	return &c
}

func (r *fakeRepo) tick() time.Time {
	r.clock = r.clock.Add(time.Minute)
	return r.clock
}

func (r *fakeRepo) Create(_ context.Context, a *applications.Application) (*applications.Application, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.items {
		if x.ListingID == a.ListingID && x.ApplicantID == a.ApplicantID && x.Status != applications.StatusWithdrawn {
			return nil, apperr.New(apperr.KindConflict, domain.MsgAlreadyApplied)
		}
	}
	now := r.tick()
	n := &applications.Application{
		ID: uuid.New(), ListingID: a.ListingID, ListingAuthorID: a.ListingAuthorID, ApplicantID: a.ApplicantID,
		Message: a.Message, Status: applications.StatusPending, CreatedAt: now, UpdatedAt: now,
	}
	r.items[n.ID] = n
	r.writes++
	return clone(n), nil
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*applications.Application, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.items[id]
	if !ok {
		return nil, apperr.New(apperr.KindNotFound, domain.MsgNotFound)
	}
	return clone(a), nil
}

func (r *fakeRepo) Update(_ context.Context, id uuid.UUID, fn func(*applications.Application) (*domain.StatusChange, error)) (*applications.Application, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.items[id]
	if !ok {
		return nil, apperr.New(apperr.KindNotFound, domain.MsgNotFound)
	}
	ch, err := fn(clone(a))
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return clone(a), nil
	}
	a.Status, a.DecidedAt, a.UpdatedAt = ch.Status, ch.DecidedAt, r.tick()
	r.writes++
	return clone(a), nil
}

func (r *fakeRepo) filter(keep func(*applications.Application) bool, status applications.Status, pendingFirst bool) []*applications.Application {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listArg = status
	out := []*applications.Application{}
	for _, a := range r.items {
		if keep(a) && (status == "" || a.Status == status) {
			out = append(out, clone(a))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		pi, pj := out[i].Status == applications.StatusPending, out[j].Status == applications.StatusPending
		if pendingFirst && pi != pj {
			return pi
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (r *fakeRepo) ListByListing(_ context.Context, id uuid.UUID, s applications.Status) ([]*applications.Application, error) {
	return r.filter(func(a *applications.Application) bool { return a.ListingID == id }, s, true), nil
}

func (r *fakeRepo) ListByApplicant(_ context.Context, id uuid.UUID, s applications.Status) ([]*applications.Application, error) {
	return r.filter(func(a *applications.Application) bool { return a.ApplicantID == id }, s, false), nil
}

func (r *fakeRepo) ListByListingAuthor(_ context.Context, id uuid.UUID, s applications.Status) ([]*applications.Application, error) {
	return r.filter(func(a *applications.Application) bool { return a.ListingAuthorID == id }, s, true), nil
}

func (r *fakeRepo) GetMineForListings(_ context.Context, applicant uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]*applications.Application, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	want := map[uuid.UUID]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := map[uuid.UUID]*applications.Application{}
	for _, a := range r.items {
		if a.ApplicantID == applicant && want[a.ListingID] && a.Status != applications.StatusWithdrawn {
			out[a.ListingID] = clone(a)
		}
	}
	return out, nil
}

func (r *fakeRepo) StatsForListings(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]applications.Stats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[uuid.UUID]applications.Stats{}
	for _, id := range ids {
		out[id] = applications.Stats{}
	}
	for _, a := range r.items {
		s, ok := out[a.ListingID]
		if !ok {
			continue
		}
		switch a.Status {
		case applications.StatusPending:
			s.Pending++
		case applications.StatusAccepted:
			s.Accepted++
		case applications.StatusDeclined:
			s.Declined++
		}
		s.Total = s.Pending + s.Accepted + s.Declined
		out[a.ListingID] = s
	}
	return out, nil
}
