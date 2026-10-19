// Package app assembles the application from its infrastructure dependencies.
// cmd/hbc builds the real dependencies; e2e tests pass blob/memory instead of S3.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/trcmkr/hse-business-club/back/internal/adapters"
	appspg "github.com/trcmkr/hse-business-club/back/internal/applications/postgres"
	appssvc "github.com/trcmkr/hse-business-club/back/internal/applications/service"
	"github.com/trcmkr/hse-business-club/back/internal/bff"
	listingspg "github.com/trcmkr/hse-business-club/back/internal/listings/postgres"
	listingssvc "github.com/trcmkr/hse-business-club/back/internal/listings/service"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
	"github.com/trcmkr/hse-business-club/back/internal/platform/server"
	"github.com/trcmkr/hse-business-club/back/internal/platform/telemetry"
	"github.com/trcmkr/hse-business-club/back/internal/seed"
	userspg "github.com/trcmkr/hse-business-club/back/internal/users/postgres"
	userssvc "github.com/trcmkr/hse-business-club/back/internal/users/service"
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
func Migrators() []db.Migrator {
	return []db.Migrator{userspg.Migrator, listingspg.Migrator, appspg.Migrator}
}

// New runs migrations, builds contexts and the BFF, seeds demo data when
// SEED_DEMO_DATA is set, and builds the server (listeners are bound). Since all of
// it happens before Run, /readyz answers only after migrations; it then checks the
// pool and the blob store.
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

	usersSvc := userssvc.New(userspg.New(d.Pool), userssvc.Bcrypt, d.Blob)
	listingsSvc := listingssvc.New(listingspg.New(d.Pool))
	applicationsSvc := appssvc.New(appspg.New(d.Pool), adapters.NewListingReader(listingsSvc))

	if cfg.Seed.DemoData {
		if _, err := seed.Run(ctx, d.Pool, d.Log.With(zap.String("component", "seed")), seed.Deps{
			Users: usersSvc, Listings: listingsSvc, Applications: applicationsSvc,
		}); err != nil {
			return nil, fmt.Errorf("app: %w", err)
		}
	}

	impl := bff.New(bff.Deps{
		Users:        usersSvc,
		Listings:     listingsSvc,
		Applications: applicationsSvc,
		Issuer:       jwt,
	})

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
