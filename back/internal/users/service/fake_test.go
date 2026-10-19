package service_test

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	"github.com/trcmkr/hse-business-club/back/internal/users/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/users/service"
)

var _ service.Repository = (*postgres.Repository)(nil)

// fakeRepo is an in-memory service.Repository with the postgres semantics.
type fakeRepo struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*users.User
	hashes map[uuid.UUID]string
	err    error // returned by every method when set
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[uuid.UUID]*users.User{}, hashes: map[uuid.UUID]string{}}
}

func clone(u *users.User) *users.User {
	c := *u
	c.Skills = slices.Clone(u.Skills)
	return &c
}

func notFound() error { return apperr.New(apperr.KindNotFound, postgres.MsgUserNotFound) }

func (r *fakeRepo) Create(_ context.Context, username, hash, displayName string) (*users.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	for _, u := range r.byID {
		if u.Username == username {
			return nil, apperr.New(apperr.KindConflict, postgres.MsgUsernameTaken)
		}
	}
	now := time.Now()
	u := &users.User{ID: uuid.New(), Username: username, DisplayName: displayName, Skills: []string{}, CreatedAt: now, UpdatedAt: now}
	r.byID[u.ID] = u
	r.hashes[u.ID] = hash
	return clone(u), nil
}

func (r *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*users.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	u, ok := r.byID[id]
	if !ok {
		return nil, notFound()
	}
	return clone(u), nil
}

func (r *fakeRepo) GetByUsername(_ context.Context, username string) (*users.User, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, "", r.err
	}
	for _, u := range r.byID {
		if u.Username == username {
			return clone(u), r.hashes[u.ID], nil
		}
	}
	return nil, "", notFound()
}

func (r *fakeRepo) GetMany(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]*users.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	out := map[uuid.UUID]*users.User{}
	for _, id := range ids {
		if u, ok := r.byID[id]; ok {
			out[id] = clone(u)
		}
	}
	return out, nil
}

func (r *fakeRepo) Update(_ context.Context, id uuid.UUID, p users.UpdateProfileParams) (*users.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	u, ok := r.byID[id]
	if !ok {
		return nil, notFound()
	}
	u.DisplayName, u.Headline, u.About, u.Skills, u.Telegram = p.DisplayName, p.Headline, p.About, slices.Clone(p.Skills), p.Telegram
	u.UpdatedAt = time.Now()
	return clone(u), nil
}

func (r *fakeRepo) SetAvatarKey(_ context.Context, id uuid.UUID, key string) (*users.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	u, ok := r.byID[id]
	if !ok {
		return nil, notFound()
	}
	u.AvatarKey = key
	u.UpdatedAt = time.Now()
	return clone(u), nil
}

// fakeHasher is a fast reversible Hasher that counts Check calls.
type fakeHasher struct {
	mu     sync.Mutex
	checks int
}

func (h *fakeHasher) Hash(p string) (string, error) { return "hash:" + p, nil }

func (h *fakeHasher) Check(hash, p string) bool {
	h.mu.Lock()
	h.checks++
	h.mu.Unlock()
	return hash == "hash:"+p
}

func (h *fakeHasher) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.checks
}
