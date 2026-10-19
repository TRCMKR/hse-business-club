package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// VersionTable is the goose version table name inside each context schema.
const VersionTable = "goose_db_version"

// Migrator applies the SQL migrations of one bounded context into its own schema.
//
// Typical use in a context package:
//
//	//go:embed migrations/*.sql
//	var migrations embed.FS
//	var Migrator = db.Migrator{Schema: "users", FS: migrations, Dir: "migrations"}
type Migrator struct {
	Schema string // Postgres schema of the context, e.g. "users"
	FS     fs.FS  // filesystem with migrations (usually embed.FS)
	Dir    string // directory inside FS, e.g. "migrations"; "" or "." = root
}

// Up creates the schema if needed and applies all pending migrations. Migrations run
// with search_path set to the schema, so they can use unqualified table names; the
// version table is <schema>.goose_db_version. A missing or empty Dir is not an error.
func (m Migrator) Up(ctx context.Context, pool *pgxpool.Pool) error {
	if m.Schema == "" {
		return errors.New("db: migrator: empty schema")
	}
	if m.FS == nil {
		return fmt.Errorf("db: migrator %s: nil FS", m.Schema)
	}
	schema := pgx.Identifier{m.Schema}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		return fmt.Errorf("db: create schema %s: %w", m.Schema, err)
	}

	fsys := m.FS
	if m.Dir != "" && m.Dir != "." {
		sub, err := fs.Sub(m.FS, m.Dir)
		if err != nil {
			return fmt.Errorf("db: migrator %s: %w", m.Schema, err)
		}
		fsys = sub
	}

	sqlDB := m.openDB(pool)
	defer sqlDB.Close()

	locker, err := lock.NewPostgresSessionLocker(lock.WithLockID(lockID(m.Schema)))
	if err != nil {
		return fmt.Errorf("db: migrator %s: locker: %w", m.Schema, err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys,
		goose.WithTableName(m.Schema+"."+VersionTable),
		goose.WithSessionLocker(locker),
		goose.WithDisableGlobalRegistry(true),
	)
	if errors.Is(err, goose.ErrNoMigrations) || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("db: migrator %s: %w", m.Schema, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("db: migrate %s: %w", m.Schema, err)
	}
	return nil
}

// openDB opens a database/sql handle on the pool's connection config with
// search_path pinned to the schema for every connection.
func (m Migrator) openDB(pool *pgxpool.Pool) *sql.DB {
	cc := pool.Config().ConnConfig.Copy()
	if cc.RuntimeParams == nil {
		cc.RuntimeParams = map[string]string{}
	}
	cc.RuntimeParams["search_path"] = m.Schema
	sqlDB := stdlib.OpenDB(*cc)
	sqlDB.SetMaxOpenConns(2)
	return sqlDB
}

// lockID derives a per-schema advisory lock id so contexts migrate independently.
func lockID(schema string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("hbc-migrate:" + schema))
	return int64(h.Sum64() >> 1) //nolint:gosec // shifted, always fits in int64
}
