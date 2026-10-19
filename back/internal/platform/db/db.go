// Package db provides the pgx connection pool, per-context schema migrations
// and a transaction helper.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
)

const (
	pingAttempts = 10
	pingInterval = time.Second
	pingTimeout  = 2 * time.Second
)

// NewPool creates a pgx pool from cfg and waits until the database answers a ping
// (up to ~10 attempts, one per second). tracer may be nil.
func NewPool(ctx context.Context, cfg config.DB, tracer pgx.QueryTracer) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db: parse dsn: %w", err)
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	}
	if tracer != nil {
		pcfg.ConnConfig.Tracer = tracer
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	var pingErr error
	for attempt := 1; attempt <= pingAttempts; attempt++ {
		pctx, cancel := context.WithTimeout(ctx, pingTimeout)
		pingErr = pool.Ping(pctx)
		cancel()
		if pingErr == nil {
			return pool, nil
		}
		if attempt == pingAttempts {
			break
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, fmt.Errorf("db: ping: %w", errors.Join(pingErr, ctx.Err()))
		case <-time.After(pingInterval):
		}
	}
	pool.Close()
	return nil, fmt.Errorf("db: ping: %w", pingErr)
}

// TxBeginner is satisfied by *pgxpool.Pool, *pgx.Conn and pgx.Tx.
type TxBeginner interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// WithTx runs fn in a transaction: commit if fn returns nil, rollback otherwise
// (including on panic, which is re-raised).
func WithTx(ctx context.Context, db TxBeginner, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return WithTxOptions(ctx, db, pgx.TxOptions{}, fn)
}

// WithTxOptions is WithTx with explicit transaction options.
func WithTxOptions(ctx context.Context, db TxBeginner, opts pgx.TxOptions, fn func(ctx context.Context, tx pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, db, opts, func(tx pgx.Tx) error {
		return fn(ctx, tx)
	})
}
