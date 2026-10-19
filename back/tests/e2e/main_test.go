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

	"github.com/trcmkr/hse-business-club/back/internal/app"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
)

// pool is shared by all tests; every test starts its own app on it.
var pool *pgxpool.Pool

// tables are truncated before every test.
const truncateSQL = "TRUNCATE applications.applications, listings.listings, users.users"

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		fmt.Println("e2e: TEST_POSTGRES_DSN is not set, skipping (use make test-e2e)")
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	var err error
	pool, err = db.NewPool(ctx, config.DB{DSN: dsn, MaxConns: 20}, nil)
	if err == nil {
		// Start from clean schemas: integration tests of the contexts may have left
		// theirs behind with other data.
		for _, mg := range app.Migrators() {
			if _, err = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+mg.Schema+" CASCADE"); err != nil {
				break
			}
		}
	}
	if err == nil {
		for _, mg := range app.Migrators() {
			if err = mg.Up(ctx, pool); err != nil {
				break
			}
		}
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
