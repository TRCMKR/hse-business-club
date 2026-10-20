package service_test

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
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

var _ service.RefreshRepository = (*postgres.RefreshRepository)(nil)

// fakeTokens is an in-memory service.RefreshRepository. InTx serializes transactions
// and rolls the state back when fn fails, like the postgres one.
type fakeTokens struct {
	tx     sync.Mutex // held for the whole InTx
	mu     sync.Mutex
	byHash map[string]*domain.StoredRefreshToken
	err    error // returned by every method except InTx when set
}

func newFakeTokens() *fakeTokens {
	return &fakeTokens{byHash: map[string]*domain.StoredRefreshToken{}}
}

func cloneToken(t *domain.StoredRefreshToken) *domain.StoredRefreshToken {
	c := *t
	c.Hash = slices.Clone(t.Hash)
	if t.RevokedAt != nil {
		at := *t.RevokedAt
		c.RevokedAt = &at
	}
	return &c
}

func (f *fakeTokens) snapshot() map[string]*domain.StoredRefreshToken {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]*domain.StoredRefreshToken, len(f.byHash))
	for k, v := range f.byHash {
		out[k] = cloneToken(v)
	}
	return out
}

// all returns copies of the stored tokens.
func (f *fakeTokens) all() []*domain.StoredRefreshToken {
	var out []*domain.StoredRefreshToken
	for _, t := range f.snapshot() {
		out = append(out, t)
	}
	return out
}

func (f *fakeTokens) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	f.tx.Lock()
	defer f.tx.Unlock()
	before := f.snapshot()
	if err := fn(ctx); err != nil {
		f.mu.Lock()
		f.byHash = before
		f.mu.Unlock()
		return err
	}
	return nil
}

func (f *fakeTokens) Create(_ context.Context, t *domain.StoredRefreshToken) (*domain.StoredRefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	if _, ok := f.byHash[string(t.Hash)]; ok {
		return nil, apperr.New(apperr.KindConflict, "Внутренняя ошибка")
	}
	c := cloneToken(t)
	c.ID, c.CreatedAt = uuid.New(), time.Now()
	f.byHash[string(t.Hash)] = c
	return cloneToken(c), nil
}

func (f *fakeTokens) GetByHash(_ context.Context, hash []byte) (*domain.StoredRefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	t, ok := f.byHash[string(hash)]
	if !ok {
		return nil, apperr.New(apperr.KindNotFound, postgres.MsgRefreshTokenNotFound)
	}
	return cloneToken(t), nil
}

func (f *fakeTokens) revokeWhere(match func(*domain.StoredRefreshToken) bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	now := time.Now()
	for _, t := range f.byHash {
		if t.RevokedAt == nil && match(t) {
			t.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeTokens) RevokeByID(_ context.Context, id uuid.UUID) error {
	return f.revokeWhere(func(t *domain.StoredRefreshToken) bool { return t.ID == id })
}

func (f *fakeTokens) RevokeFamily(_ context.Context, familyID uuid.UUID) error {
	return f.revokeWhere(func(t *domain.StoredRefreshToken) bool { return t.FamilyID == familyID })
}

func (f *fakeTokens) DeleteExpired(_ context.Context, userID uuid.UUID, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	for k, t := range f.byHash {
		if t.UserID == userID && !t.ExpiresAt.After(now) {
			delete(f.byHash, k)
		}
	}
	return nil
}
