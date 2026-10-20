//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob/memory"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	"github.com/trcmkr/hse-business-club/back/internal/users/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/users/service"
)

// TestRotateRefreshRace: two concurrent rotations of one refresh token on the real
// service and Postgres. GetByHash locks the row, so exactly one wins; the other sees
// the token already rotated, which is a reuse: 401 and the whole family is revoked,
// including the token the winner has just received (SPEC §2.3, §5).
func TestRotateRefreshRace(t *testing.T) {
	repo, pool := setup(t)
	svc := service.New(repo, postgres.NewRefresh(pool), service.Bcrypt, memory.New())
	ctx := context.Background()

	u, err := repo.Create(ctx, "anna", "hash", "Анна")
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.IssueRefresh(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		userID uuid.UUID
		next   *users.RefreshToken
		err    error
	}
	results := make([]result, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range results {
		wg.Go(func() {
			<-start
			id, next, err := svc.RotateRefresh(ctx, first.Token)
			results[i] = result{id, next, err}
		})
	}
	close(start)
	wg.Wait()

	var winner *users.RefreshToken
	for _, r := range results {
		switch {
		case r.err == nil:
			if winner != nil {
				t.Fatal("both rotations succeeded")
			}
			if r.userID != u.ID || r.next == nil || r.next.Token == first.Token {
				t.Fatalf("rotation = %v %+v", r.userID, r.next)
			}
			winner = r.next
		case apperr.KindOf(r.err) == apperr.KindUnauthenticated:
			wantKind(t, r.err, apperr.KindUnauthenticated, service.MsgSessionExpired)
		default:
			t.Fatalf("unexpected error: %v", r.err)
		}
	}
	if winner == nil {
		t.Fatalf("no rotation succeeded: %v, %v", results[0].err, results[1].err)
	}

	// The family is revoked: the winner's token does not work either.
	_, _, err = svc.RotateRefresh(ctx, winner.Token)
	wantKind(t, err, apperr.KindUnauthenticated, service.MsgSessionExpired)
	var active int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users.refresh_tokens WHERE revoked_at IS NULL`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("%d tokens of the family are still active", active)
	}
}
