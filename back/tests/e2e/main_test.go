//go:build e2e

// Package e2e runs HTTP end-to-end scenarios (SPEC §1, §2.3) against the whole
// application started in-process: real Postgres (TEST_POSTGRES_DSN), the real gRPC
// server and grpc-gateway on free ports, blob/memory instead of S3.
package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db/dbtest"
)

// pool is shared by all tests; every test starts its own app on it.
var pool *pgxpool.Pool

// tables are truncated before every test.
const truncateSQL = "TRUNCATE applications.applications, listings.listings, users.refresh_tokens, users.users"

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		fmt.Println("e2e: TEST_POSTGRES_DSN is not set, skipping (use make test-e2e)")
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	// Same lock as the integration tests (dbtest), then a clean database migrated
	// from back/migrations: integration tests may have left data behind.
	err := dbtest.Lock(ctx, dsn)
	if err == nil {
		pool, err = db.NewPool(ctx, config.DB{DSN: dsn, MaxConns: 20}, nil)
	}
	if err == nil {
		err = dbtest.Reset(ctx, pool)
	}
	if err == nil {
		err = dbtest.Up(ctx, pool)
	}
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: setup:", err)
		os.Exit(1)
	}
	code := m.Run()
	pool.Close()
	os.Exit(code)
}
