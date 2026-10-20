//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
	"github.com/trcmkr/hse-business-club/back/internal/users/postgres"
)

func TestRefreshRepository(t *testing.T) {
	users, pool := setup(t)
	repo := postgres.NewRefresh(pool)
	ctx := context.Background()

	newUser := func(t *testing.T, name string) uuid.UUID {
		t.Helper()
		u, err := users.Create(ctx, name, "hash", name)
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	create := func(t *testing.T, userID, family uuid.UUID, expires time.Time) (*domain.StoredRefreshToken, string) {
		t.Helper()
		token, hash, err := domain.NewRefreshToken()
		if err != nil {
			t.Fatal(err)
		}
		st, err := repo.Create(ctx, &domain.StoredRefreshToken{UserID: userID, FamilyID: family, Hash: hash, ExpiresAt: expires})
		if err != nil {
			t.Fatal(err)
		}
		return st, token
	}
	get := func(t *testing.T, token string) *domain.StoredRefreshToken {
		t.Helper()
		st, err := repo.GetByHash(ctx, domain.HashRefreshToken(token))
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	future := time.Now().Add(time.Hour).Truncate(time.Microsecond)

	t.Run("create and get by hash", func(t *testing.T) {
		truncate(t, pool)
		uid, fam := newUser(t, "anna"), uuid.New()
		st, token := create(t, uid, fam, future)
		if st.ID == uuid.Nil || st.UserID != uid || st.FamilyID != fam || !st.ExpiresAt.Equal(future) || st.RevokedAt != nil || st.CreatedAt.IsZero() {
			t.Fatalf("created %+v", st)
		}
		if got := get(t, token); got.ID != st.ID {
			t.Fatalf("GetByHash = %+v", got)
		}
		_, err := repo.GetByHash(ctx, domain.HashRefreshToken("unknown"))
		wantKind(t, err, apperr.KindNotFound, postgres.MsgRefreshTokenNotFound)
	})

	t.Run("hash is unique", func(t *testing.T) {
		truncate(t, pool)
		uid := newUser(t, "anna")
		st, _ := create(t, uid, uuid.New(), future)
		_, err := repo.Create(ctx, &domain.StoredRefreshToken{UserID: uid, FamilyID: uuid.New(), Hash: st.Hash, ExpiresAt: future})
		wantKind(t, err, apperr.KindConflict, "")
	})

	t.Run("only the hash is stored", func(t *testing.T) {
		truncate(t, pool)
		_, token := create(t, newUser(t, "anna"), uuid.New(), future)
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM users.refresh_tokens t WHERE t::text LIKE '%' || $1 || '%'`, token).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatal("plain token found in the table")
		}
	})

	t.Run("revoke by id and family", func(t *testing.T) {
		truncate(t, pool)
		uid, fam := newUser(t, "anna"), uuid.New()
		a, ta := create(t, uid, fam, future)
		_, tb := create(t, uid, fam, future)
		_, tc := create(t, uid, uuid.New(), future) // another family

		if err := repo.RevokeByID(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
		revokedA := get(t, ta).RevokedAt
		if revokedA == nil || get(t, tb).RevokedAt != nil {
			t.Fatal("RevokeByID must revoke exactly one token")
		}
		if err := repo.RevokeByID(ctx, uuid.New()); err != nil { // missing: not an error
			t.Fatal(err)
		}

		if err := repo.RevokeFamily(ctx, fam); err != nil {
			t.Fatal(err)
		}
		if get(t, tb).RevokedAt == nil || get(t, tc).RevokedAt != nil {
			t.Fatal("RevokeFamily must revoke the family only")
		}
		if !get(t, ta).RevokedAt.Equal(*revokedA) {
			t.Fatal("an already revoked token keeps its revoked_at")
		}
	})

	t.Run("delete expired of the user", func(t *testing.T) {
		truncate(t, pool)
		anna, boris := newUser(t, "anna"), newUser(t, "boris")
		now := time.Now()
		_, expired := create(t, anna, uuid.New(), now.Add(-time.Minute))
		_, alive := create(t, anna, uuid.New(), future)
		_, foreign := create(t, boris, uuid.New(), now.Add(-time.Minute))

		if err := repo.DeleteExpired(ctx, anna, now); err != nil {
			t.Fatal(err)
		}
		_, err := repo.GetByHash(ctx, domain.HashRefreshToken(expired))
		wantKind(t, err, apperr.KindNotFound, "")
		get(t, alive)
		get(t, foreign)
	})

	t.Run("tokens go with the user", func(t *testing.T) {
		truncate(t, pool)
		uid := newUser(t, "anna")
		_, token := create(t, uid, uuid.New(), future)
		if _, err := pool.Exec(ctx, `DELETE FROM users.users WHERE id = $1`, uid); err != nil {
			t.Fatal(err)
		}
		_, err := repo.GetByHash(ctx, domain.HashRefreshToken(token))
		wantKind(t, err, apperr.KindNotFound, "")
	})

	t.Run("in tx: commit, rollback, nested", func(t *testing.T) {
		truncate(t, pool)
		uid := newUser(t, "anna")
		st, token := create(t, uid, uuid.New(), future)
		boom := errors.New("boom")

		err := repo.InTx(ctx, func(ctx context.Context) error {
			if err := repo.RevokeByID(ctx, st.ID); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("InTx = %v, want fn error as is", err)
		}
		if get(t, token).RevokedAt != nil {
			t.Fatal("rollback did not undo the revocation")
		}

		err = repo.InTx(ctx, func(ctx context.Context) error {
			return repo.InTx(ctx, func(ctx context.Context) error { return repo.RevokeByID(ctx, st.ID) })
		})
		if err != nil {
			t.Fatal(err)
		}
		if get(t, token).RevokedAt == nil {
			t.Fatal("commit lost the revocation")
		}
	})

	t.Run("get by hash locks the row in tx", func(t *testing.T) {
		truncate(t, pool)
		_, token := create(t, newUser(t, "anna"), uuid.New(), future)
		hash := domain.HashRefreshToken(token)
		locked, release := make(chan struct{}), make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- repo.InTx(ctx, func(ctx context.Context) error {
				if _, err := repo.GetByHash(ctx, hash); err != nil {
					return err
				}
				close(locked)
				<-release
				return nil
			})
		}()
		<-locked
		waited := make(chan time.Duration, 1)
		go func() {
			start := time.Now()
			_ = repo.InTx(ctx, func(ctx context.Context) error {
				_, err := repo.GetByHash(ctx, hash)
				return err
			})
			waited <- time.Since(start)
		}()
		time.Sleep(200 * time.Millisecond)
		close(release)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if d := <-waited; d < 150*time.Millisecond {
			t.Fatalf("second GetByHash did not wait for the lock (%s)", d)
		}
	})
}
