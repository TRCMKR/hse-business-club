// Package app assembles the application from its infrastructure dependencies.
// cmd/hbc builds the real dependencies; e2e tests (B5) can pass e.g. blob/memory.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/trcmkr/hse-business-club/back/internal/bff"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
	"github.com/trcmkr/hse-business-club/back/internal/platform/server"
	"github.com/trcmkr/hse-business-club/back/internal/platform/telemetry"
)

// Deps are the infrastructure dependencies owned by the caller: the caller closes
// the pool and shuts telemetry down after Run returns.
type Deps struct {
	Log       *zap.Logger
	Telemetry *telemetry.Telemetry
	Pool      *pgxpool.Pool
	Blob      blob.Store
}

// App is the assembled application.
type App struct {
	log    *zap.Logger
	server *server.Server
	bff    *bff.Implementation
}

// Migrators returns the migrators of all bounded contexts, in order.
// B1–B3 add theirs here: users, listings, applications.
func Migrators() []db.Migrator {
	return []db.Migrator{}
}

// New runs migrations, builds contexts, the BFF and the server (listeners are bound).
func New(ctx context.Context, cfg *config.Config, d Deps) (*App, error) {
	if d.Log == nil || d.Telemetry == nil || d.Pool == nil || d.Blob == nil {
		return nil, errors.New("app: Log, Telemetry, Pool and Blob are required")
	}

	if err := d.Telemetry.Register(telemetry.PoolCollector(d.Pool)); err != nil {
		return nil, err
	}

	for _, m := range Migrators() {
		if err := m.Up(ctx, d.Pool); err != nil {
			return nil, fmt.Errorf("app: %w", err)
		}
		d.Log.Info("migrations applied", zap.String("schema", m.Schema))
	}

	jwt := auth.NewJWT(cfg.Auth.JWTSecret, cfg.Auth.JWTTTL)

	// Contexts are wired here by B1–B4 (users.Service needs d.Blob for photos;
	// applications.ListingReader comes from internal/adapters).
	impl := bff.New(bff.Deps{
		Issuer: jwt,
	})

	if cfg.Seed.DemoData {
		d.Log.Warn("SEED_DEMO_DATA is set, but seeding is not implemented yet")
	}

	srv, err := server.New(ctx, cfg.Server, d.Log, d.Telemetry, jwt, impl)
	if err != nil {
		return nil, err
	}
	srv.AddReadinessCheck("db", d.Pool.Ping)
	srv.AddReadinessCheck("blob", d.Blob.Ping)

	return &App{log: d.Log, server: srv, bff: impl}, nil
}

// Server returns the server (handlers and bound addresses, for tests).
func (a *App) Server() *server.Server {
	return a.server
}

// Run serves until ctx is cancelled, then shuts the server down gracefully.
func (a *App) Run(ctx context.Context) error {
	return a.server.Run(ctx)
}
