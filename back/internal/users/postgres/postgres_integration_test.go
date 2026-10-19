//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db/dbtest"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	"github.com/trcmkr/hse-business-club/back/internal/users/postgres"
)

// setup recreates schema "users" for the test; subtests call truncate to start clean.
func setup(t *testing.T) (*postgres.Repository, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Migrate(t, pool, postgres.Migrator)
	return postgres.New(pool), pool
}

func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE users.users"); err != nil {
		t.Fatal(err)
	}
}

func wantKind(t *testing.T, err error, kind apperr.Kind, msg string) {
	t.Helper()
	if apperr.KindOf(err) != kind {
		t.Fatalf("want %v, got %v", kind, err)
	}
	var e *apperr.Error
	if msg != "" && (!errors.As(err, &e) || e.Msg != msg) {
		t.Fatalf("want message %q, got %v", msg, err)
	}
}

func TestRepository(t *testing.T) {
	repo, pool := setup(t)
	ctx := context.Background()

	t.Run("create and get", func(t *testing.T) {
		truncate(t, pool)
		before := time.Now().Add(-time.Minute)
		u, err := repo.Create(ctx, "anna", "hash-1", "Анна")
		if err != nil {
			t.Fatal(err)
		}
		if u.ID == uuid.Nil || u.Username != "anna" || u.DisplayName != "Анна" ||
			u.Skills == nil || len(u.Skills) != 0 || u.AvatarKey != "" || u.CreatedAt.Before(before) {
			t.Fatalf("unexpected user %+v", u)
		}

		got, err := repo.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, u) {
			t.Fatalf("GetByID %+v, want %+v", got, u)
		}

		got, hash, err := repo.GetByUsername(ctx, "anna")
		if err != nil {
			t.Fatal(err)
		}
		if hash != "hash-1" || !reflect.DeepEqual(got, u) {
			t.Fatalf("GetByUsername %+v %q", got, hash)
		}
	})

	t.Run("duplicate username", func(t *testing.T) {
		truncate(t, pool)
		if _, err := repo.Create(ctx, "anna", "h", "A"); err != nil {
			t.Fatal(err)
		}
		_, err := repo.Create(ctx, "anna", "h2", "B")
		wantKind(t, err, apperr.KindConflict, postgres.MsgUsernameTaken)
	})

	t.Run("not found", func(t *testing.T) {
		truncate(t, pool)
		_, err := repo.GetByID(ctx, uuid.New())
		wantKind(t, err, apperr.KindNotFound, postgres.MsgUserNotFound)
		_, _, err = repo.GetByUsername(ctx, "nobody")
		wantKind(t, err, apperr.KindNotFound, postgres.MsgUserNotFound)
		_, err = repo.Update(ctx, uuid.New(), users.UpdateProfileParams{DisplayName: "X"})
		wantKind(t, err, apperr.KindNotFound, postgres.MsgUserNotFound)
		_, err = repo.SetAvatarKey(ctx, uuid.New(), "k.jpg")
		wantKind(t, err, apperr.KindNotFound, postgres.MsgUserNotFound)
	})

	t.Run("get many", func(t *testing.T) {
		truncate(t, pool)
		a, _ := repo.Create(ctx, "a_user", "h", "A")
		b, _ := repo.Create(ctx, "b_user", "h", "B")
		got, err := repo.GetMany(ctx, []uuid.UUID{a.ID, b.ID, uuid.New(), a.ID})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[a.ID].Username != "a_user" || got[b.ID].Username != "b_user" {
			t.Fatalf("GetMany %+v", got)
		}
		empty, err := repo.GetMany(ctx, nil)
		if err != nil || empty == nil || len(empty) != 0 {
			t.Fatalf("GetMany(nil) = %v, %v", empty, err)
		}
	})

	t.Run("update", func(t *testing.T) {
		truncate(t, pool)
		u, _ := repo.Create(ctx, "anna", "h", "Анна")
		p := users.UpdateProfileParams{
			DisplayName: "Анна К.", Headline: "Продакт", About: "Про меня",
			Skills: []string{"Go", "Маркетинг"}, Telegram: "anna_k",
		}
		got, err := repo.Update(ctx, u.ID, p)
		if err != nil {
			t.Fatal(err)
		}
		if got.DisplayName != p.DisplayName || got.Headline != p.Headline || got.About != p.About ||
			!reflect.DeepEqual(got.Skills, p.Skills) || got.Telegram != p.Telegram ||
			got.Username != "anna" || !got.CreatedAt.Equal(u.CreatedAt) || got.UpdatedAt.Before(u.UpdatedAt) {
			t.Fatalf("Update %+v", got)
		}
		got, err = repo.Update(ctx, u.ID, users.UpdateProfileParams{DisplayName: "A"})
		if err != nil {
			t.Fatal(err)
		}
		if got.Skills == nil || len(got.Skills) != 0 || got.Headline != "" {
			t.Fatalf("full replacement expected, got %+v", got)
		}
	})

	t.Run("avatar key", func(t *testing.T) {
		truncate(t, pool)
		u, _ := repo.Create(ctx, "anna", "h", "Анна")
		key := uuid.NewString() + ".jpg"
		got, err := repo.SetAvatarKey(ctx, u.ID, key)
		if err != nil || got.AvatarKey != key {
			t.Fatalf("SetAvatarKey %+v %v", got, err)
		}
		got, err = repo.SetAvatarKey(ctx, u.ID, "")
		if err != nil || got.AvatarKey != "" {
			t.Fatalf("SetAvatarKey('') %+v %v", got, err)
		}
	})
}

// Up on an already migrated schema is a no-op, and the table lives in schema "users".
func TestMigrationIdempotent(t *testing.T) {
	repo, pool := setup(t)
	ctx := context.Background()
	if _, err := repo.Create(ctx, "keep_me", "h", "K"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := postgres.Migrator.Up(ctx, pool); err != nil {
			t.Fatalf("second Up: %v", err)
		}
	}
	if _, _, err := repo.GetByUsername(ctx, "keep_me"); err != nil {
		t.Fatalf("data lost after re-migration: %v", err)
	}
	var versions int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users.goose_db_version WHERE version_id = 1").Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 1 {
		t.Fatalf("version 1 recorded %d times", versions)
	}
	var schema string
	if err := pool.QueryRow(ctx, "SELECT table_schema FROM information_schema.tables WHERE table_name = 'users' AND table_schema = 'users'").Scan(&schema); err != nil {
		t.Fatalf("users.users not found: %v", err)
	}
}
