//go:build integration

package db_test

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"

	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db/dbtest"
)

func TestMigratorAndTx(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	m := db.Migrator{
		Schema: "b0_test",
		Dir:    "migrations",
		FS: fstest.MapFS{
			"migrations/00001_init.sql": {Data: []byte(
				"-- +goose Up\nCREATE TABLE items (id int PRIMARY KEY, name text NOT NULL);\n" +
					"-- +goose Down\nDROP TABLE items;\n")},
		},
	}
	dbtest.Migrate(t, pool, m)

	// Idempotent second run.
	if err := m.Up(ctx, pool); err != nil {
		t.Fatalf("second Up: %v", err)
	}

	var version int64
	if err := pool.QueryRow(ctx, "SELECT max(version_id) FROM b0_test.goose_db_version").Scan(&version); err != nil {
		t.Fatalf("version table: %v", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}

	// Table landed in the context schema, not in public.
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('b0_test.items') IS NOT NULL").Scan(&exists); err != nil || !exists {
		t.Fatalf("b0_test.items must exist: %v", err)
	}

	// Commit.
	err := db.WithTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO b0_test.items VALUES (1, 'a')")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	// Rollback.
	boom := errors.New("boom")
	err = db.WithTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO b0_test.items VALUES (2, 'b')"); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}

	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM b0_test.items").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows = %d, want 1 (second insert rolled back)", n)
	}
}

func TestMigratorEmpty(t *testing.T) {
	pool := dbtest.Pool(t)
	m := db.Migrator{Schema: "b0_empty", FS: fstest.MapFS{}, Dir: "migrations"}
	t.Cleanup(func() { dbtest.ResetSchema(t, pool, m.Schema) })
	if err := m.Up(context.Background(), pool); err != nil {
		t.Fatalf("empty migrator must be a no-op: %v", err)
	}
}
