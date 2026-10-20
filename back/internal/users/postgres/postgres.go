// Package postgres is the users context storage: schema "users" in PostgreSQL.
package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
)

// Messages of errors returned by the repository.
const (
	MsgUsernameTaken = "Логин уже занят"
	MsgUserNotFound  = "Участник не найден"
	msgInternal      = "Внутренняя ошибка"
)

const uniqueViolation = "23505"

const columns = `id, username, display_name, headline, about, skills, telegram, avatar_key, created_at, updated_at`

// Repository stores users in users.users. All errors are *apperr.Error:
// KindNotFound, KindConflict, or KindUnknown wrapping the database error.
type Repository struct {
	pool *pgxpool.Pool
}

// New returns a repository on the pool.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a user. A taken username is KindConflict «Логин уже занят».
func (r *Repository) Create(ctx context.Context, username, passwordHash, displayName string) (*users.User, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users.users (username, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING `+columns,
		username, passwordHash, displayName)
	u, err := scanUser(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil, apperr.Wrap(apperr.KindConflict, MsgUsernameTaken, err)
		}
		return nil, wrap(err)
	}
	return u, nil
}

// GetByID returns a user, KindNotFound if there is none.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*users.User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+columns+` FROM users.users WHERE id = $1`, id)
	u, err := scanUser(row)
	if err != nil {
		return nil, wrap(err)
	}
	return u, nil
}

// GetByUsername returns a user and its password hash, KindNotFound if there is none.
// The username must already be normalized.
func (r *Repository) GetByUsername(ctx context.Context, username string) (*users.User, string, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+columns+`, password_hash FROM users.users WHERE username = $1`, username)
	var (
		u    users.User
		hash string
	)
	if err := row.Scan(userDest(&u, &hash)...); err != nil {
		return nil, "", wrap(err)
	}
	fixSkills(&u)
	return &u, hash, nil
}

// GetMany returns users by id; missing ids are absent from the map.
func (r *Repository) GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*users.User, error) {
	out := make(map[uuid.UUID]*users.User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM users.users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out[u.ID] = u
	}
	if err := rows.Err(); err != nil {
		return nil, wrap(err)
	}
	return out, nil
}

// Update replaces the editable profile fields, KindNotFound if there is no user.
// The params must already be normalized.
func (r *Repository) Update(ctx context.Context, id uuid.UUID, p users.UpdateProfileParams) (*users.User, error) {
	skills := p.Skills
	if skills == nil {
		skills = []string{}
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE users.users
		SET display_name = $2, headline = $3, about = $4, skills = $5, telegram = $6, updated_at = now()
		WHERE id = $1
		RETURNING `+columns,
		id, p.DisplayName, p.Headline, p.About, skills, p.Telegram)
	u, err := scanUser(row)
	if err != nil {
		return nil, wrap(err)
	}
	return u, nil
}

// SetAvatarKey sets the photo key ("" = no photo), KindNotFound if there is no user.
func (r *Repository) SetAvatarKey(ctx context.Context, id uuid.UUID, key string) (*users.User, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE users.users SET avatar_key = $2, updated_at = now()
		WHERE id = $1
		RETURNING `+columns,
		id, key)
	u, err := scanUser(row)
	if err != nil {
		return nil, wrap(err)
	}
	return u, nil
}

func scanUser(row pgx.Row) (*users.User, error) {
	var u users.User
	if err := row.Scan(userDest(&u)...); err != nil {
		return nil, err
	}
	fixSkills(&u)
	return &u, nil
}

// userDest returns scan destinations in the order of columns, followed by extra.
func userDest(u *users.User, extra ...any) []any {
	return append([]any{
		&u.ID, &u.Username, &u.DisplayName, &u.Headline, &u.About,
		&u.Skills, &u.Telegram, &u.AvatarKey, &u.CreatedAt, &u.UpdatedAt,
	}, extra...)
}

func fixSkills(u *users.User) {
	if u.Skills == nil {
		u.Skills = []string{}
	}
}

func wrap(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.Wrap(apperr.KindNotFound, MsgUserNotFound, err)
	}
	return apperr.Wrap(apperr.KindUnknown, msgInternal, err)
}
