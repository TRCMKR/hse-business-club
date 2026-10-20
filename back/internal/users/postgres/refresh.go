package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
)

// MsgRefreshTokenNotFound is the (internal) message of a missing refresh token; the
// service turns it into «Сессия истекла».
const MsgRefreshTokenNotFound = "Refresh-токен не найден" //nolint:gosec // message, not a credential

const refreshColumns = `id, user_id, family_id, token_hash, expires_at, created_at, revoked_at`

// querier is what both *pgxpool.Pool and pgx.Tx provide.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// RefreshRepository stores refresh tokens in users.refresh_tokens. All errors are
// *apperr.Error: KindNotFound, KindConflict (duplicate hash) or KindUnknown.
type RefreshRepository struct {
	pool *pgxpool.Pool
}

// NewRefresh returns a refresh token repository on the pool.
func NewRefresh(pool *pgxpool.Pool) *RefreshRepository {
	return &RefreshRepository{pool: pool}
}

type txKey struct{}

// q returns the transaction started by InTx for this ctx, or the pool.
func (r *RefreshRepository) q(ctx context.Context) querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return r.pool
}

// InTx runs fn in one transaction: every method of r called with the ctx passed to fn
// uses it. Commit if fn returns nil, rollback otherwise; a nested InTx joins the outer
// transaction. Errors of fn are returned as is.
func (r *RefreshRepository) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	var fnErr error
	err := db.WithTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		fnErr = fn(context.WithValue(ctx, txKey{}, tx))
		return fnErr
	})
	if err != nil && fnErr == nil {
		return apperr.Wrap(apperr.KindUnknown, msgInternal, err)
	}
	return err
}

// Create inserts a token. A duplicate hash is KindConflict (never happens with
// 256-bit random tokens).
func (r *RefreshRepository) Create(ctx context.Context, t *domain.StoredRefreshToken) (*domain.StoredRefreshToken, error) {
	row := r.q(ctx).QueryRow(ctx, `
		INSERT INTO users.refresh_tokens (user_id, family_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING `+refreshColumns,
		t.UserID, t.FamilyID, t.Hash, t.ExpiresAt)
	out, err := scanRefresh(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil, apperr.Wrap(apperr.KindConflict, msgInternal, err)
		}
		return nil, wrapRefresh(err)
	}
	return out, nil
}

// GetByHash returns the token with this hash (active, revoked or expired),
// KindNotFound if there is none. The row is locked FOR UPDATE: inside InTx
// concurrent rotations of one token run one after another.
func (r *RefreshRepository) GetByHash(ctx context.Context, hash []byte) (*domain.StoredRefreshToken, error) {
	row := r.q(ctx).QueryRow(ctx, `SELECT `+refreshColumns+` FROM users.refresh_tokens WHERE token_hash = $1 FOR UPDATE`, hash)
	t, err := scanRefresh(row)
	if err != nil {
		return nil, wrapRefresh(err)
	}
	return t, nil
}

// RevokeByID revokes one token; an already revoked or missing token is not an error.
func (r *RefreshRepository) RevokeByID(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx, `UPDATE users.refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	return wrapRefresh(err)
}

// RevokeFamily revokes every active token of the family.
func (r *RefreshRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx, `UPDATE users.refresh_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`, familyID)
	return wrapRefresh(err)
}

// DeleteExpired deletes the user's tokens that expired before now (revoked ones too:
// an expired token is rejected anyway, reuse detection is not needed for it).
func (r *RefreshRepository) DeleteExpired(ctx context.Context, userID uuid.UUID, now time.Time) error {
	_, err := r.q(ctx).Exec(ctx, `DELETE FROM users.refresh_tokens WHERE user_id = $1 AND expires_at <= $2`, userID, now)
	return wrapRefresh(err)
}

func scanRefresh(row pgx.Row) (*domain.StoredRefreshToken, error) {
	var t domain.StoredRefreshToken
	if err := row.Scan(&t.ID, &t.UserID, &t.FamilyID, &t.Hash, &t.ExpiresAt, &t.CreatedAt, &t.RevokedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

func wrapRefresh(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.Wrap(apperr.KindNotFound, MsgRefreshTokenNotFound, err)
	}
	return apperr.Wrap(apperr.KindUnknown, msgInternal, err)
}
