// Command hbc is the backend of the «Стартапы и вакансии» module: the hbc.v1 BFF
// over gRPC and HTTP/JSON (grpc-gateway).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/trcmkr/hse-business-club/back/internal/app"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob/s3"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
	"github.com/trcmkr/hse-business-club/back/internal/platform/telemetry"
)

const startupTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hbc:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()
	logger.SetGlobal(log)
	log.Info("starting",
		zap.String("version", telemetry.Version),
		zap.String("commit", telemetry.Commit),
		zap.Any("config", cfg.Redacted()),
	)

	tel, err := telemetry.Init(ctx, cfg.Telemetry)
	if err != nil {
		return err
	}
	shutdownTelemetry := func() {
		sctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		if err := tel.Shutdown(sctx); err != nil {
			log.Warn("telemetry shutdown", zap.Error(err))
		}
	}

	startCtx, cancelStart := context.WithTimeout(ctx, startupTimeout)
	defer cancelStart()

	pool, err := db.NewPool(startCtx, cfg.DB, tel.PgxTracer())
	if err != nil {
		shutdownTelemetry()
		return err
	}
	// Shutdown order (ARCHITECTURE §4): server (inside Run) → flush traces → close pool.
	defer func() {
		shutdownTelemetry()
		pool.Close()
		log.Info("stopped")
	}()

	store, err := s3.New(startCtx, cfg.Blob, s3.WithRegisterer(tel.Registry()))
	if err != nil {
		return err
	}
	log.Info("blob storage ready", zap.String("bucket", store.Bucket()))

	a, err := app.New(startCtx, cfg, app.Deps{Log: log, Telemetry: tel, Pool: pool, Blob: store})
	if err != nil {
		return err
	}
	cancelStart()

	return a.Run(ctx)
}
