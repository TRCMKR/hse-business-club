// Package postgres stores applications in the "applications" Postgres schema.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/applications/domain"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Schema is the Postgres schema of the applications context.
const Schema = "applications"

// Migrator applies the applications context migrations.
var Migrator = db.Migrator{Schema: Schema, FS: migrations, Dir: "migrations"}

const (
	uniqueViolation = "23505"
	activeUniqIndex = "applications_active_uniq"

	columns = `id, listing_id, listing_author_id, applicant_id, message, status, created_at, updated_at, decided_at`

	// pending first, then newest first; id makes the order total.
	orderPendingFirst = ` ORDER BY (status = 'pending') DESC, created_at DESC, id DESC`
	orderNewestFirst  = ` ORDER BY created_at DESC, id DESC`
)

// Repo is the applications repository on a pgx pool.
type Repo struct {
	pool *pgxpool.Pool
}

// New creates a repository on pool.
func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// StatusChange is a status update computed under the row lock.
type StatusChange struct {
	Status    applications.Status
	DecidedAt *time.Time
}

// Create inserts a new pending application from ListingID, ListingAuthorID, ApplicantID
// and Message of a; the other fields are assigned by the database.
// A live (not withdrawn) application of the same applicant to the same listing →
// KindConflict «Вы уже откликались на эту публикацию».
func (r *Repo) Create(ctx context.Context, a *applications.Application) (*applications.Application, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO applications.applications (listing_id, listing_author_id, applicant_id, message)
		VALUES ($1, $2, $3, $4)
		RETURNING `+columns,
		a.ListingID, a.ListingAuthorID, a.ApplicantID, a.Message)
	created, err := scan(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == activeUniqIndex {
			return nil, apperr.Wrap(apperr.KindConflict, domain.MsgAlreadyApplied, err)
		}
		return nil, fmt.Errorf("applications: create: %w", err)
	}
	return created, nil
}

// GetByID returns the application, KindNotFound «Отклик не найден» if there is none.
func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*applications.Application, error) {
	return getOne(r.pool.QueryRow(ctx, `SELECT `+columns+` FROM applications.applications WHERE id = $1`, id))
}

// GetForUpdate returns the application locking its row until tx ends. KindNotFound if there is none.
func (r *Repo) GetForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*applications.Application, error) {
	return getOne(tx.QueryRow(ctx, `SELECT `+columns+` FROM applications.applications WHERE id = $1 FOR UPDATE`, id))
}

// UpdateStatus sets status and decided_at (nil — NULL), bumps updated_at and returns the new row.
func (r *Repo) UpdateStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status applications.Status, decidedAt *time.Time) (*applications.Application, error) {
	return getOne(tx.QueryRow(ctx, `
		UPDATE applications.applications
		SET status = $2, decided_at = $3, updated_at = now()
		WHERE id = $1
		RETURNING `+columns,
		id, string(status), decidedAt))
}

// Update locks the application row, passes the current state to fn and, if fn returns
// a non-nil change, applies it — all in one transaction. A nil change means no-op: the
// current state is returned without writing. Errors of fn abort the transaction as is.
func (r *Repo) Update(ctx context.Context, id uuid.UUID,
	fn func(cur *applications.Application) (*StatusChange, error),
) (*applications.Application, error) {
	var result *applications.Application
	err := db.WithTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		cur, err := r.GetForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		change, err := fn(cur)
		if err != nil {
			return err
		}
		if change == nil {
			result = cur
			return nil
		}
		result, err = r.UpdateStatus(ctx, tx, id, change.Status, change.DecidedAt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListByListing returns applications to a listing, pending first, then newest first.
// status "" means all statuses.
func (r *Repo) ListByListing(ctx context.Context, listingID uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	return r.list(ctx, `listing_id = $1`, orderPendingFirst, listingID, status)
}

// ListByApplicant returns applications sent by applicant, newest first. status "" means all.
func (r *Repo) ListByApplicant(ctx context.Context, applicantID uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	return r.list(ctx, `applicant_id = $1`, orderNewestFirst, applicantID, status)
}

// ListByListingAuthor returns applications to all listings of author, pending first,
// then newest first. status "" means all.
func (r *Repo) ListByListingAuthor(ctx context.Context, authorID uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	return r.list(ctx, `listing_author_id = $1`, orderPendingFirst, authorID, status)
}

func (r *Repo) list(ctx context.Context, where, order string, id uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+columns+` FROM applications.applications
		WHERE `+where+` AND ($2 = '' OR status = $2)`+order,
		id, string(status))
	if err != nil {
		return nil, fmt.Errorf("applications: list: %w", err)
	}
	return collect(rows)
}

// GetMineForListings returns the live (not withdrawn) application of applicant for each
// of listingIDs that has one. The unique partial index guarantees at most one per listing.
func (r *Repo) GetMineForListings(ctx context.Context, applicantID uuid.UUID, listingIDs []uuid.UUID) (map[uuid.UUID]*applications.Application, error) {
	out := make(map[uuid.UUID]*applications.Application, len(listingIDs))
	if len(listingIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+columns+` FROM applications.applications
		WHERE applicant_id = $1 AND status <> 'withdrawn' AND listing_id = ANY($2)`,
		applicantID, listingIDs)
	if err != nil {
		return nil, fmt.Errorf("applications: get mine: %w", err)
	}
	list, err := collect(rows)
	if err != nil {
		return nil, err
	}
	for _, a := range list {
		out[a.ListingID] = a
	}
	return out, nil
}

// StatsForListings counts applications per listing and status. Every requested id is
// present in the result; listings without applications get zero Stats.
// Total = Pending + Accepted + Declined (withdrawn are not counted, SPEC §4.2).
func (r *Repo) StatsForListings(ctx context.Context, listingIDs []uuid.UUID) (map[uuid.UUID]applications.Stats, error) {
	out := make(map[uuid.UUID]applications.Stats, len(listingIDs))
	for _, id := range listingIDs {
		out[id] = applications.Stats{}
	}
	if len(listingIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT listing_id, status, count(*) FROM applications.applications
		WHERE listing_id = ANY($1)
		GROUP BY listing_id, status`,
		listingIDs)
	if err != nil {
		return nil, fmt.Errorf("applications: stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id     uuid.UUID
			status string
			n      int
		)
		if err := rows.Scan(&id, &status, &n); err != nil {
			return nil, fmt.Errorf("applications: stats: scan: %w", err)
		}
		s := out[id]
		switch applications.Status(status) {
		case applications.StatusPending:
			s.Pending += n
		case applications.StatusAccepted:
			s.Accepted += n
		case applications.StatusDeclined:
			s.Declined += n
		}
		s.Total = s.Pending + s.Accepted + s.Declined
		out[id] = s
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: stats: %w", err)
	}
	return out, nil
}

func scan(row pgx.Row) (*applications.Application, error) {
	var (
		a      applications.Application
		status string
	)
	if err := row.Scan(&a.ID, &a.ListingID, &a.ListingAuthorID, &a.ApplicantID, &a.Message,
		&status, &a.CreatedAt, &a.UpdatedAt, &a.DecidedAt); err != nil {
		return nil, err
	}
	a.Status = applications.Status(status)
	return &a, nil
}

func getOne(row pgx.Row) (*applications.Application, error) {
	a, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Wrap(apperr.KindNotFound, domain.MsgNotFound, err)
	}
	if err != nil {
		return nil, fmt.Errorf("applications: get: %w", err)
	}
	return a, nil
}

func collect(rows pgx.Rows) ([]*applications.Application, error) {
	defer rows.Close()
	out := make([]*applications.Application, 0)
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("applications: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: rows: %w", err)
	}
	return out, nil
}
