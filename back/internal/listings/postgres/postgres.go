// Package postgres is the listings repository on PostgreSQL (schema "listings").
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/listings/domain"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
)

// Schema is the Postgres schema of the listings context.
const Schema = "listings"

//go:embed migrations/*.sql
var migrations embed.FS

// Migrator applies the listings migrations into the "listings" schema.
var Migrator = db.Migrator{Schema: Schema, FS: migrations, Dir: "migrations"}

// Queries use schema-qualified names: the runtime pool keeps the default search_path.
const (
	columns = `id, author_id, kind, title, description, looking_for, tags,
		COALESCE(stage, ''), compensation, status, created_at, updated_at, closed_at`

	// Ties on created_at are broken by id so pagination is stable.
	order = ` ORDER BY created_at DESC, id DESC`

	// $1 kind ('' = all), $2 ILIKE pattern ('' = no search).
	feedWhere = ` FROM listings.listings
		WHERE status = 'active'
		  AND ($1 = '' OR kind = $1)
		  AND ($2 = '' OR title ILIKE $2 ESCAPE '\' OR description ILIKE $2 ESCAPE '\')`
)

// Repository stores listings. Methods return domain.ErrNotFound() for a missing id.
type Repository struct {
	pool *pgxpool.Pool
}

// New creates a repository on the pool.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a listing with status active. p must be normalized by domain.
func (r *Repository) Create(ctx context.Context, p listings.CreateParams) (*listings.Listing, error) {
	row := r.pool.QueryRow(ctx, `INSERT INTO listings.listings
		(author_id, kind, title, description, looking_for, tags, stage, compensation)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
		RETURNING `+columns,
		p.AuthorID, string(p.Kind), p.Title, p.Description, p.LookingFor, tagsOrEmpty(p.Tags),
		string(p.Stage), p.Compensation)
	l, err := scan(row)
	if err != nil {
		return nil, fmt.Errorf("listings: create: %w", err)
	}
	return l, nil
}

// GetByID returns the listing in any status.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*listings.Listing, error) {
	l, err := scan(r.pool.QueryRow(ctx, `SELECT `+columns+` FROM listings.listings WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound()
	}
	if err != nil {
		return nil, fmt.Errorf("listings: get: %w", err)
	}
	return l, nil
}

// GetMany returns the found listings by id; missing ids are absent from the map.
func (r *Repository) GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*listings.Listing, error) {
	out := make(map[uuid.UUID]*listings.Listing, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM listings.listings WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("listings: get many: %w", err)
	}
	items, err := collect(rows)
	if err != nil {
		return nil, fmt.Errorf("listings: get many: %w", err)
	}
	for _, l := range items {
		out[l.ID] = l
	}
	return out, nil
}

// ListActive returns one page of active listings (newest first) and the total number
// of matches. f must be normalized by domain. The count and the page are read in one
// read-only repeatable-read transaction, so total is consistent with items even for a
// page past the end (empty items, real total).
func (r *Repository) ListActive(ctx context.Context, f listings.ListFilter) ([]*listings.Listing, int, error) {
	pattern := ""
	if f.Query != "" {
		pattern = "%" + EscapeLike(f.Query) + "%"
	}
	var (
		items []*listings.Listing
		total int
	)
	opts := pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	err := db.WithTxOptions(ctx, r.pool, opts, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*)`+feedWhere, string(f.Kind), pattern).Scan(&total); err != nil {
			return err
		}
		if total == 0 {
			items = []*listings.Listing{}
			return nil
		}
		rows, err := tx.Query(ctx, `SELECT `+columns+feedWhere+order+` LIMIT $3 OFFSET $4`,
			string(f.Kind), pattern, f.Limit, domain.Offset(f))
		if err != nil {
			return err
		}
		items, err = collect(rows)
		return err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listings: list active: %w", err)
	}
	return items, total, nil
}

// ListByAuthor returns the author's listings, newest first; status "" = all.
func (r *Repository) ListByAuthor(ctx context.Context, authorID uuid.UUID, status listings.Status) ([]*listings.Listing, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM listings.listings
		WHERE author_id = $1 AND ($2 = '' OR status = $2)`+order, authorID, string(status))
	if err != nil {
		return nil, fmt.Errorf("listings: list by author: %w", err)
	}
	items, err := collect(rows)
	if err != nil {
		return nil, fmt.Errorf("listings: list by author: %w", err)
	}
	return items, nil
}

// Update replaces the editable fields of an active listing. p must be normalized by
// domain. It returns domain.ErrNotActive when the listing is missing or not active.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, p listings.UpdateParams) (*listings.Listing, error) {
	row := r.pool.QueryRow(ctx, `UPDATE listings.listings SET
		title = $2, description = $3, looking_for = $4, tags = $5,
		stage = NULLIF($6, ''), compensation = $7, updated_at = now()
		WHERE id = $1 AND status = 'active'
		RETURNING `+columns,
		id, p.Title, p.Description, p.LookingFor, tagsOrEmpty(p.Tags), string(p.Stage), p.Compensation)
	l, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotActive
	}
	if err != nil {
		return nil, fmt.Errorf("listings: update: %w", err)
	}
	return l, nil
}

// Close moves an active listing to closed. It returns domain.ErrNotActive when the
// listing is missing or already closed.
func (r *Repository) Close(ctx context.Context, id uuid.UUID) (*listings.Listing, error) {
	row := r.pool.QueryRow(ctx, `UPDATE listings.listings SET
		status = 'closed', closed_at = now(), updated_at = now()
		WHERE id = $1 AND status = 'active'
		RETURNING `+columns, id)
	l, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotActive
	}
	if err != nil {
		return nil, fmt.Errorf("listings: close: %w", err)
	}
	return l, nil
}

// EscapeLike escapes the LIKE metacharacters \, % and _ (escape character '\').
func EscapeLike(s string) string {
	return likeEscaper.Replace(s)
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func scan(row pgx.Row) (*listings.Listing, error) {
	var (
		l                   listings.Listing
		kind, stage, status string
	)
	if err := row.Scan(&l.ID, &l.AuthorID, &kind, &l.Title, &l.Description, &l.LookingFor, &l.Tags,
		&stage, &l.Compensation, &status, &l.CreatedAt, &l.UpdatedAt, &l.ClosedAt); err != nil {
		return nil, err
	}
	l.Kind, l.Stage, l.Status = listings.Kind(kind), listings.Stage(stage), listings.Status(status)
	l.Tags = tagsOrEmpty(l.Tags)
	return &l, nil
}

func collect(rows pgx.Rows) ([]*listings.Listing, error) {
	defer rows.Close()
	out := []*listings.Listing{}
	for rows.Next() {
		l, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func tagsOrEmpty(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
