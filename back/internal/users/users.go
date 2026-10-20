// Package users is the users bounded context: accounts, profiles and profile photos.
//
// This file is the port from ARCHITECTURE §2: users/service implements Service,
// the BFF consumes it. Signatures change only together with ARCHITECTURE.md.
package users

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// User is a club member profile.
type User struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
	Headline    string
	About       string
	Skills      []string
	Telegram    string
	AvatarKey   string // "<uuid>.jpg" в хранилище; "" — фото нет
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RegisterParams are the inputs of Service.Register.
type RegisterParams struct{ Username, Password, DisplayName string }

// Avatar — готовый к отдаче файл фото.
type Avatar struct {
	ContentType string // всегда "image/jpeg"
	Data        []byte
}

// UpdateProfileParams are the editable profile fields (full replacement).
type UpdateProfileParams struct {
	DisplayName, Headline, About string
	Skills                       []string
	Telegram                     string
}

// Service is the users context API. Errors are *apperr.Error.
type Service interface {
	// KindConflict «Логин уже занят», KindValidation.
	Register(ctx context.Context, p RegisterParams) (*User, error)
	// KindUnauthenticated «Неверный логин или пароль» (одно сообщение на оба случая).
	Authenticate(ctx context.Context, username, password string) (*User, error)
	// KindNotFound.
	Get(ctx context.Context, id uuid.UUID) (*User, error)
	// Отсутствующие id просто не попадают в map.
	GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*User, error)
	// KindNotFound, KindValidation.
	UpdateProfile(ctx context.Context, id uuid.UUID, p UpdateProfileParams) (*User, error)
	// image — исходный файл (JPEG/PNG/WebP, ≤ 5 МБ). Нормализует до 512×512 JPEG, кладёт
	// в хранилище под новым ключом, старый удаляет. KindNotFound, KindValidation «Фото: …».
	SetAvatar(ctx context.Context, id uuid.UUID, image []byte) (*User, error)
	// Идемпотентно. KindNotFound.
	RemoveAvatar(ctx context.Context, id uuid.UUID) (*User, error)
	// key — значение User.AvatarKey. KindNotFound «Фото не найдено» (и при плохом формате key).
	OpenAvatar(ctx context.Context, key string) (*Avatar, error)

	// Refresh-токены (SPEC §4.2, §5). Выдаёт новую семью токенов для входа.
	IssueRefresh(ctx context.Context, userID uuid.UUID) (*RefreshToken, error)
	// Ротация: token отзывается, выдаётся следующий в той же семье. KindUnauthenticated
	// «Сессия истекла» при неизвестном/истёкшем/отозванном; повторное использование уже
	// ротированного токена отзывает всю семью.
	RotateRefresh(ctx context.Context, token string) (userID uuid.UUID, next *RefreshToken, err error)
	// Отзывает всю семью токена (выход). Идемпотентно: неизвестный токен — не ошибка.
	RevokeRefresh(ctx context.Context, token string) error
}

// RefreshToken is an issued refresh token.
type RefreshToken struct {
	Token     string    // непрозрачный, base64url(32 случайных байта); наружу — только через BFF
	ExpiresAt time.Time
}
