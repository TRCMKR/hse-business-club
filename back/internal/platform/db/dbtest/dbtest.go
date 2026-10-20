// Package dbtest helps integration and e2e tests get a pool on the test database
// (TEST_POSTGRES_DSN, see docker-compose.test.yaml) and a schema migrated from
// back/migrations with the goose library: the same files the goose CLI applies.
//
// All test packages share one database, one set of schemas and one goose version
// table, so a test process takes a database-wide advisory lock before touching it:
// packages that use the database run one after another (`go test ./...` runs
// packages in parallel), the others are not affected.
package dbtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
)

// EnvDSN is the environment variable with the test database DSN.
const EnvDSN = "TEST_POSTGRES_DSN"

// Schemas are the schemas created by back/migrations; Reset drops them.
var Schemas = []string{"users", "listings", "applications"}

// lockKey is the advisory lock that serializes test processes on the database.
const lockKey int64 = 0x6862635f74657374 // "hbc_test"

var (
	lockOnce sync.Once
	// lockConn keeps the locking connection reachable: an unreferenced socket would be
	// closed by its finalizer and the lock released. It is closed by the process exit.
	lockConn *pgx.Conn //nolint:unused // write-only on purpose, see above
	errLock  error
)

// MigrationsDir returns the absolute path of back/migrations, derived from this
// source file rather than from the working directory (it differs per test package).
func MigrationsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("dbtest: runtime.Caller failed")
	}
	// file is back/internal/platform/db/dbtest/dbtest.go
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "migrations")
}

// Lock takes the database-wide test lock once per process and holds it until the
// process exits. Pool calls it; tests with their own TestMain (e2e) call it directly.
func Lock(ctx context.Context, dsn string) error {
	lockOnce.Do(func() {
		conn, err := pgx.Connect(ctx, dsn)
		if err != nil {
			errLock = fmt.Errorf("dbtest: lock: %w", err)
			return
		}
		if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
			_ = conn.Close(ctx)
			errLock = fmt.Errorf("dbtest: lock: %w", err)
			return
		}
		lockConn = conn
	})
	return errLock
}

// Pool takes the test lock and returns a pool on TEST_POSTGRES_DSN, closed on test
// cleanup. The test is skipped when the variable is not set.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		t.Skipf("%s is not set", EnvDSN)
	}
	// Waiting for other packages' tests to release the lock counts against this timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := Lock(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := db.NewPool(ctx, config.DB{DSN: dsn, MaxConns: 5}, nil)
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Up applies all pending migrations from back/migrations; the version table is
// public.goose_db_version, as with the goose CLI.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDB(*pool.Config().ConnConfig.Copy())
	defer func() { _ = sqlDB.Close() }()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, os.DirFS(MigrationsDir()),
		goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return fmt.Errorf("dbtest: goose: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("dbtest: goose up: %w", err)
	}
	return nil
}

// Reset drops all context schemas and the goose version table, so the next Up
// starts from an empty database.
func Reset(ctx context.Context, pool *pgxpool.Pool) error {
	for _, s := range Schemas {
		if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{s}.Sanitize()+" CASCADE"); err != nil {
			return fmt.Errorf("dbtest: drop schema %s: %w", s, err)
		}
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS public.goose_db_version"); err != nil {
		return fmt.Errorf("dbtest: drop version table: %w", err)
	}
	return nil
}

// Migrate resets the database and applies all migrations, registering a cleanup
// that resets it again.
func Migrate(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if err := Reset(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Reset(context.Background(), pool); err != nil {
			t.Error(err)
		}
	})
}
