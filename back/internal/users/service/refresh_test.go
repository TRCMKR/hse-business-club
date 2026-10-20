package service_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	blobmocks "github.com/trcmkr/hse-business-club/back/internal/platform/blob/mocks"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
	"github.com/trcmkr/hse-business-club/back/internal/users/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/users/service"
	"github.com/trcmkr/hse-business-club/back/internal/users/service/mocks"
)

// txResult records how the fn passed to RefreshRepository.InTx ended: nil commits,
// an error rolls back.
type txResult struct {
	called bool
	err    error
}

// expectTx mocks InTx like the postgres one: fn runs with the given ctx and its error
// is returned as is.
func (e *env) expectTx() *txResult {
	res := &txResult{}
	e.tokens.EXPECT().InTx(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			res.called = true
			res.err = fn(ctx)
			return res.err
		}).Once()
	return res
}

// expectCreate expects one stored token and records it; the repository fills ID and
// CreatedAt.
func (e *env) expectCreate(err error) *[]*domain.StoredRefreshToken {
	var stored []*domain.StoredRefreshToken
	e.tokens.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, t *domain.StoredRefreshToken) (*domain.StoredRefreshToken, error) {
			if err != nil {
				return nil, err
			}
			c := *t
			c.ID, c.CreatedAt = uuid.New(), e.now
			stored = append(stored, &c)
			return &c, nil
		})
	return &stored
}

func notFoundToken() error { return apperr.New(apperr.KindNotFound, postgres.MsgRefreshTokenNotFound) }

// token returns a stored token valid for another hour.
func (e *env) token() *domain.StoredRefreshToken {
	return &domain.StoredRefreshToken{
		ID: uuid.New(), UserID: uuid.New(), FamilyID: uuid.New(), Hash: []byte("hash"),
		ExpiresAt: e.now.Add(time.Hour), CreatedAt: e.now.Add(-time.Hour),
	}
}

func TestIssueRefresh(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	userID := uuid.New()
	stored := e.expectCreate(nil)

	a, err := e.svc.IssueRefresh(ctx, userID)
	require.NoError(t, err)
	b, err := e.svc.IssueRefresh(ctx, userID)
	require.NoError(t, err)

	require.Len(t, *stored, 2)
	sa, sb := (*stored)[0], (*stored)[1]
	assert.Len(t, a.Token, domain.RefreshTokenLen)
	assert.NotEqual(t, a.Token, b.Token)
	assert.True(t, a.ExpiresAt.Equal(e.now.Add(testRefreshTTL)), "ExpiresAt = %s, want now + %s", a.ExpiresAt, testRefreshTTL)
	assert.True(t, sa.ExpiresAt.Equal(a.ExpiresAt))
	assert.Equal(t, userID, sa.UserID)
	assert.Nil(t, sa.RevokedAt)
	assert.NotEqual(t, uuid.Nil, sa.FamilyID)
	assert.NotEqual(t, sa.FamilyID, sb.FamilyID, "each sign-in is a new family")
	// Only hashes are stored.
	assert.Equal(t, domain.HashRefreshToken(a.Token), sa.Hash)
	assert.Equal(t, domain.HashRefreshToken(b.Token), sb.Hash)
	for _, st := range *stored {
		assert.False(t, bytes.Contains(st.Hash, []byte(a.Token)) || bytes.Contains(st.Hash, []byte(b.Token)), "plain token stored")
	}
}

func TestRefreshTTLOption(t *testing.T) {
	for name, ttl := range map[string]time.Duration{"zero": 0, "negative": -time.Hour} {
		t.Run(name+" is ignored", func(t *testing.T) {
			tokens, hasher := mocks.NewMockRefreshRepository(t), mocks.NewMockHasher(t)
			hasher.EXPECT().Hash(mock.Anything).Return(dummyHash, nil).Once()
			now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
			svc := service.New(mocks.NewMockRepository(t), tokens, hasher, blobmocks.NewMockStore(t),
				service.WithRefreshTTL(ttl), service.WithClock(func() time.Time { return now }))
			tokens.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, st *domain.StoredRefreshToken) (*domain.StoredRefreshToken, error) {
					return st, nil
				}).Once()

			rt, err := svc.IssueRefresh(context.Background(), uuid.New())
			require.NoError(t, err)
			assert.True(t, rt.ExpiresAt.Equal(now.Add(service.DefaultRefreshTTL)))
		})
	}
}

func TestRotateRefresh(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	const old = "old-token"
	cur := e.token()

	tx := e.expectTx()
	get := e.tokens.EXPECT().GetByHash(mock.Anything, domain.HashRefreshToken(old)).Return(cur, nil).Once()
	revoke := e.tokens.EXPECT().RevokeByID(mock.Anything, cur.ID).Return(nil).Once()
	// the user's expired tokens are dropped on rotation
	cleanup := e.tokens.EXPECT().DeleteExpired(mock.Anything, cur.UserID, e.now).Return(nil).Once()
	stored := e.expectCreate(nil)
	mock.InOrder(get, revoke, cleanup)

	userID, next, err := e.svc.RotateRefresh(ctx, old)
	require.NoError(t, err)
	require.True(t, tx.called)
	require.NoError(t, tx.err, "committed")
	assert.Equal(t, cur.UserID, userID)
	assert.NotEqual(t, old, next.Token)
	require.Len(t, *stored, 1)
	st := (*stored)[0]
	assert.Equal(t, cur.FamilyID, st.FamilyID, "the next token is in the same family")
	assert.Equal(t, cur.UserID, st.UserID)
	assert.Equal(t, domain.HashRefreshToken(next.Token), st.Hash)
	assert.True(t, next.ExpiresAt.Equal(e.now.Add(testRefreshTTL)))
}

// Reuse of a revoked (already rotated) token revokes the whole family; the revocation
// is committed (fn returns nil) and the caller gets 401.
func TestRotateRefreshReuseRevokesFamily(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	cur := e.token()
	revokedAt := e.now.Add(-time.Minute)
	cur.RevokedAt = &revokedAt

	tx := e.expectTx()
	e.tokens.EXPECT().GetByHash(mock.Anything, domain.HashRefreshToken("rotated")).Return(cur, nil).Once()
	e.tokens.EXPECT().RevokeFamily(mock.Anything, cur.FamilyID).Return(nil).Once()
	// no RevokeByID / DeleteExpired / Create

	userID, next, err := e.svc.RotateRefresh(ctx, "rotated")
	wantErr(t, err, apperr.KindUnauthenticated, service.MsgSessionExpired)
	assert.Equal(t, uuid.Nil, userID)
	assert.Nil(t, next)
	require.True(t, tx.called)
	assert.NoError(t, tx.err, "the family revocation must be committed")
}

func TestRotateRefreshRejects(t *testing.T) {
	ctx := context.Background()

	t.Run("empty: no storage access", func(t *testing.T) {
		e := newEnv(t)
		_, _, err := e.svc.RotateRefresh(ctx, "")
		wantErr(t, err, apperr.KindUnauthenticated, service.MsgSessionExpired)
	})
	t.Run("unknown", func(t *testing.T) {
		e := newEnv(t)
		token := strings.Repeat("A", domain.RefreshTokenLen)
		tx := e.expectTx()
		e.tokens.EXPECT().GetByHash(mock.Anything, domain.HashRefreshToken(token)).Return(nil, notFoundToken()).Once()
		_, _, err := e.svc.RotateRefresh(ctx, token)
		wantErr(t, err, apperr.KindUnauthenticated, service.MsgSessionExpired)
		assert.Error(t, tx.err, "rolled back")
	})
	t.Run("expired: left as is (rollback)", func(t *testing.T) {
		e := newEnv(t)
		cur := e.token()
		cur.ExpiresAt = e.now // expires_at == now is already expired
		tx := e.expectTx()
		e.tokens.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(cur, nil).Once()
		// no RevokeByID / RevokeFamily / Create
		_, _, err := e.svc.RotateRefresh(ctx, "expired")
		wantErr(t, err, apperr.KindUnauthenticated, service.MsgSessionExpired)
		assert.Error(t, tx.err, "rolled back")
	})
}

func TestRotateRefreshStorageFailure(t *testing.T) {
	ctx := context.Background()

	t.Run("transaction", func(t *testing.T) {
		e := newEnv(t)
		e.tokens.EXPECT().InTx(mock.Anything, mock.Anything).Return(dbDown()).Once()
		_, _, err := e.svc.RotateRefresh(ctx, "token")
		wantErr(t, err, apperr.KindUnknown, "")
	})
	t.Run("get by hash", func(t *testing.T) {
		e := newEnv(t)
		e.expectTx()
		e.tokens.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(nil, dbDown()).Once()
		_, _, err := e.svc.RotateRefresh(ctx, "token")
		wantErr(t, err, apperr.KindUnknown, "")
	})
	t.Run("revoke family on reuse", func(t *testing.T) {
		e := newEnv(t)
		cur := e.token()
		cur.RevokedAt = &e.now
		e.expectTx()
		e.tokens.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(cur, nil).Once()
		e.tokens.EXPECT().RevokeFamily(mock.Anything, cur.FamilyID).Return(dbDown()).Once()
		_, _, err := e.svc.RotateRefresh(ctx, "token")
		wantErr(t, err, apperr.KindUnknown, "")
	})
	t.Run("revoke by id", func(t *testing.T) {
		e := newEnv(t)
		cur := e.token()
		tx := e.expectTx()
		e.tokens.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(cur, nil).Once()
		e.tokens.EXPECT().RevokeByID(mock.Anything, cur.ID).Return(dbDown()).Once()
		_, _, err := e.svc.RotateRefresh(ctx, "token")
		wantErr(t, err, apperr.KindUnknown, "")
		assert.Error(t, tx.err, "rolled back")
	})
	t.Run("delete expired", func(t *testing.T) {
		e := newEnv(t)
		cur := e.token()
		tx := e.expectTx()
		e.tokens.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(cur, nil).Once()
		e.tokens.EXPECT().RevokeByID(mock.Anything, cur.ID).Return(nil).Once()
		e.tokens.EXPECT().DeleteExpired(mock.Anything, cur.UserID, e.now).Return(dbDown()).Once()
		_, _, err := e.svc.RotateRefresh(ctx, "token")
		wantErr(t, err, apperr.KindUnknown, "")
		assert.Error(t, tx.err, "rolled back")
	})
	t.Run("create next", func(t *testing.T) {
		e := newEnv(t)
		cur := e.token()
		tx := e.expectTx()
		e.tokens.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(cur, nil).Once()
		e.tokens.EXPECT().RevokeByID(mock.Anything, cur.ID).Return(nil).Once()
		e.tokens.EXPECT().DeleteExpired(mock.Anything, cur.UserID, e.now).Return(nil).Once()
		e.expectCreate(dbDown())
		userID, next, err := e.svc.RotateRefresh(ctx, "token")
		wantErr(t, err, apperr.KindUnknown, "")
		assert.Equal(t, uuid.Nil, userID)
		assert.Nil(t, next)
		assert.Error(t, tx.err, "rolled back")
	})
	t.Run("issue", func(t *testing.T) {
		e := newEnv(t)
		e.expectCreate(dbDown())
		_, err := e.svc.IssueRefresh(ctx, uuid.New())
		wantErr(t, err, apperr.KindUnknown, "")
	})
}

func TestRevokeRefresh(t *testing.T) {
	ctx := context.Background()

	// Logout with any token of the sign-in (active or already rotated) ends the sign-in.
	t.Run("known token revokes its family", func(t *testing.T) {
		e := newEnv(t)
		active, rotated := e.token(), e.token()
		rotated.RevokedAt = &e.now
		for token, cur := range map[string]*domain.StoredRefreshToken{"active": active, "rotated": rotated} {
			e.tokens.EXPECT().GetByHash(mock.Anything, domain.HashRefreshToken(token)).Return(cur, nil).Once()
			e.tokens.EXPECT().RevokeFamily(mock.Anything, cur.FamilyID).Return(nil).Once()
			require.NoError(t, e.svc.RevokeRefresh(ctx, token))
		}
	})
	t.Run("empty and unknown are not an error", func(t *testing.T) {
		e := newEnv(t)
		require.NoError(t, e.svc.RevokeRefresh(ctx, "")) // no storage access
		e.tokens.EXPECT().GetByHash(mock.Anything, domain.HashRefreshToken("unknown")).Return(nil, notFoundToken()).Once()
		require.NoError(t, e.svc.RevokeRefresh(ctx, "unknown"))
	})
	t.Run("storage failure", func(t *testing.T) {
		e := newEnv(t)
		e.tokens.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(nil, dbDown()).Once()
		wantErr(t, e.svc.RevokeRefresh(ctx, "token"), apperr.KindUnknown, "")

		cur := e.token()
		e.tokens.EXPECT().GetByHash(mock.Anything, mock.Anything).Return(cur, nil).Once()
		e.tokens.EXPECT().RevokeFamily(mock.Anything, cur.FamilyID).Return(dbDown()).Once()
		wantErr(t, e.svc.RevokeRefresh(ctx, "token"), apperr.KindUnknown, "")
	})
}
