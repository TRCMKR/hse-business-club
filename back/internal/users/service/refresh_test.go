package service_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
	"github.com/trcmkr/hse-business-club/back/internal/users/service"
)

func (e *env) issue(t *testing.T, userID uuid.UUID) *users.RefreshToken {
	t.Helper()
	rt, err := e.svc.IssueRefresh(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

func (e *env) stored(t *testing.T, token string) *domain.StoredRefreshToken {
	t.Helper()
	st, err := e.tokens.GetByHash(context.Background(), domain.HashRefreshToken(token))
	if err != nil {
		t.Fatalf("token is not stored: %v", err)
	}
	return st
}

func TestIssueRefresh(t *testing.T) {
	e := newEnv()
	u := e.register(t, testUser)
	a, b := e.issue(t, u.ID), e.issue(t, u.ID)

	if len(a.Token) != domain.RefreshTokenLen || a.Token == b.Token {
		t.Fatalf("tokens %q, %q", a.Token, b.Token)
	}
	if !a.ExpiresAt.Equal(e.now.Add(testRefreshTTL)) {
		t.Fatalf("ExpiresAt = %s, want now + %s", a.ExpiresAt, testRefreshTTL)
	}
	sa, sb := e.stored(t, a.Token), e.stored(t, b.Token)
	if sa.UserID != u.ID || sa.RevokedAt != nil || sa.FamilyID == sb.FamilyID {
		t.Fatalf("stored = %+v / %+v: each sign-in is a new family", sa, sb)
	}
	// Only hashes are stored.
	for _, st := range e.tokens.all() {
		if bytes.Contains(st.Hash, []byte(a.Token)) || bytes.Contains(st.Hash, []byte(b.Token)) {
			t.Fatal("plain token stored")
		}
	}
}

func TestRotateRefresh(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	u := e.register(t, testUser)
	first := e.issue(t, u.ID)

	userID, next, err := e.svc.RotateRefresh(ctx, first.Token)
	if err != nil {
		t.Fatal(err)
	}
	if userID != u.ID || next.Token == first.Token {
		t.Fatalf("RotateRefresh = %s, %q", userID, next.Token)
	}
	old, cur := e.stored(t, first.Token), e.stored(t, next.Token)
	if old.RevokedAt == nil || cur.RevokedAt != nil || old.FamilyID != cur.FamilyID {
		t.Fatalf("old %+v, next %+v: the old one is revoked, the next one is in the same family", old, cur)
	}

	// The next token rotates again.
	if _, _, err := e.svc.RotateRefresh(ctx, next.Token); err != nil {
		t.Fatal(err)
	}
}

func TestRotateRefreshReuseRevokesFamily(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	u := e.register(t, testUser)
	first := e.issue(t, u.ID)
	other := e.issue(t, u.ID) // another sign-in of the same user
	_, next, err := e.svc.RotateRefresh(ctx, first.Token)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = e.svc.RotateRefresh(ctx, first.Token)
	wantErr(t, err, apperr.KindUnauthenticated, service.MsgSessionExpired)
	if e.stored(t, next.Token).RevokedAt == nil {
		t.Fatal("reuse must revoke the whole family (the revocation is committed)")
	}
	_, _, err = e.svc.RotateRefresh(ctx, next.Token)
	wantErr(t, err, apperr.KindUnauthenticated, service.MsgSessionExpired)

	// Other sign-ins are not affected.
	if _, _, err := e.svc.RotateRefresh(ctx, other.Token); err != nil {
		t.Fatalf("another family: %v", err)
	}
}

func TestRotateRefreshRejects(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	u := e.register(t, testUser)
	expired := e.issue(t, u.ID)
	e.now = e.now.Add(testRefreshTTL)

	for name, token := range map[string]string{
		"empty": "", "unknown": strings.Repeat("A", domain.RefreshTokenLen), "expired": expired.Token,
	} {
		_, _, err := e.svc.RotateRefresh(ctx, token)
		if err == nil {
			t.Fatalf("%s: rotated", name)
		}
		wantErr(t, err, apperr.KindUnauthenticated, service.MsgSessionExpired)
	}
	if e.stored(t, expired.Token).RevokedAt != nil {
		t.Fatal("a rejected expired token is left as is (rollback)")
	}
}

func TestRotateRefreshDeletesExpired(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	u := e.register(t, testUser)
	stale := e.issue(t, u.ID)
	e.now = e.now.Add(testRefreshTTL - time.Minute)
	fresh := e.issue(t, u.ID)
	e.now = e.now.Add(time.Minute)

	if _, _, err := e.svc.RotateRefresh(ctx, fresh.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := e.tokens.GetByHash(ctx, domain.HashRefreshToken(stale.Token)); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("expired token must be deleted, got %v", err)
	}
	if n := len(e.tokens.all()); n != 2 {
		t.Fatalf("%d tokens left, want 2 (rotated + next)", n)
	}
}

func TestRotateRefreshStorageFailure(t *testing.T) {
	e := newEnv()
	u := e.register(t, testUser)
	rt := e.issue(t, u.ID)
	e.tokens.err = apperr.Wrap(apperr.KindUnknown, "Внутренняя ошибка", errors.New("db down"))

	_, _, err := e.svc.RotateRefresh(context.Background(), rt.Token)
	wantErr(t, err, apperr.KindUnknown, "")
	_, err = e.svc.IssueRefresh(context.Background(), u.ID)
	wantErr(t, err, apperr.KindUnknown, "")
	err = e.svc.RevokeRefresh(context.Background(), rt.Token)
	wantErr(t, err, apperr.KindUnknown, "")
}

// Concurrent rotations of one token: exactly one wins, the other is a reuse and
// revokes the family, so the winner's token is dead too.
func TestRotateRefreshConcurrent(t *testing.T) {
	e := newEnv()
	u := e.register(t, testUser)
	rt := e.issue(t, u.ID)

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins []*users.RefreshToken
	)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, next, err := e.svc.RotateRefresh(context.Background(), rt.Token); err == nil {
				mu.Lock()
				wins = append(wins, next)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(wins) != 1 {
		t.Fatalf("%d rotations succeeded, want 1", len(wins))
	}
	if e.stored(t, wins[0].Token).RevokedAt == nil {
		t.Fatal("the losing rotation is a reuse: the family must be revoked")
	}
}

func TestRevokeRefresh(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	u := e.register(t, testUser)
	first := e.issue(t, u.ID)
	_, next, err := e.svc.RotateRefresh(ctx, first.Token)
	if err != nil {
		t.Fatal(err)
	}

	// Logout with any token of the sign-in ends it; repeating is fine.
	for range 2 {
		if err := e.svc.RevokeRefresh(ctx, first.Token); err != nil {
			t.Fatal(err)
		}
	}
	if e.stored(t, next.Token).RevokedAt == nil {
		t.Fatal("logout must revoke the sign-in")
	}
	_, _, err = e.svc.RotateRefresh(ctx, next.Token)
	wantErr(t, err, apperr.KindUnauthenticated, service.MsgSessionExpired)

	for _, token := range []string{"", "unknown"} {
		if err := e.svc.RevokeRefresh(ctx, token); err != nil {
			t.Fatalf("RevokeRefresh(%q) = %v, want nil", token, err)
		}
	}
}
