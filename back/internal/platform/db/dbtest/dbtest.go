// Package dbtest helps integration tests get a pool on the test database
// (TEST_POSTGRES_DSN, see docker-compose.test.yaml).
package dbtest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
)

// EnvDSN is the environment variable with the test database DSN.
const EnvDSN = "TEST_POSTGRES_DSN"

// Pool returns a pool on TEST_POSTGRES_DSN, closed on test cleanup.
// The test is skipped when the variable is not set.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		t.Skipf("%s is not set", EnvDSN)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, config.DB{DSN: dsn, MaxConns: 5}, nil)
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// ResetSchema drops the schema (with all its tables and the goose version table)
// so the next Migrator.Up starts from scratch.
func ResetSchema(t testing.TB, pool *pgxpool.Pool, schema string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	if err != nil {
		t.Fatalf("dbtest: drop schema %s: %v", schema, err)
	}
}

// Migrate resets the schema and applies the migrator, registering a cleanup that drops it again.
func Migrate(t testing.TB, pool *pgxpool.Pool, m db.Migrator) {
	t.Helper()
	ResetSchema(t, pool, m.Schema)
	if err := m.Up(context.Background(), pool); err != nil {
		t.Fatalf("dbtest: migrate %s: %v", m.Schema, err)
	}
	t.Cleanup(func() { ResetSchema(t, pool, m.Schema) })
}
