//go:build integration

package db_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db/dbtest"
)

// back/migrations: up is idempotent, every Down section works, up again restores the schema.
func TestMigrations(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	dbtest.Migrate(t, pool)

	if err := dbtest.Up(ctx, pool); err != nil {
		t.Fatalf("second Up: %v", err)
	}
	var applied int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.goose_db_version WHERE version_id > 0 AND is_applied").Scan(&applied); err != nil {
		t.Fatalf("version table: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(dbtest.MigrationsDir(), "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("migration files: %v %v", files, err)
	}
	if applied != len(files) {
		t.Fatalf("applied migrations = %d, want %d", applied, len(files))
	}
	tablesExist := func() bool {
		t.Helper()
		var ok bool
		err := pool.QueryRow(ctx, `SELECT to_regclass('users.users') IS NOT NULL
			AND to_regclass('users.refresh_tokens') IS NOT NULL
			AND to_regclass('listings.listings') IS NOT NULL
			AND to_regclass('applications.applications') IS NOT NULL`).Scan(&ok)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !tablesExist() {
		t.Fatal("tables of the contexts must exist after Up")
	}

	sqlDB := stdlib.OpenDB(*pool.Config().ConnConfig.Copy())
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, os.DirFS(dbtest.MigrationsDir()),
		goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("down to 0: %v", err)
	}
	var schemas int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_namespace WHERE nspname IN ('users', 'listings', 'applications')").Scan(&schemas); err != nil {
		t.Fatal(err)
	}
	if schemas != 0 {
		t.Fatalf("%d context schemas left after down to 0", schemas)
	}
	if err := dbtest.Up(ctx, pool); err != nil {
		t.Fatalf("Up after down: %v", err)
	}
	if !tablesExist() {
		t.Fatal("tables of the contexts must exist after down + up")
	}
}

func TestWithTx(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	for _, q := range []string{
		"DROP SCHEMA IF EXISTS b0_test CASCADE",
		"CREATE SCHEMA b0_test",
		"CREATE TABLE b0_test.items (id int PRIMARY KEY, name text NOT NULL)",
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS b0_test CASCADE") })

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
