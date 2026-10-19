// Package auth provides password hashing, JWT issuing/verification and the gRPC
// authentication interceptor.
package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// BcryptCost is the bcrypt cost for password hashes (SPEC §5).
const BcryptCost = 10

// MsgUnauthenticated is the user-facing message for missing or invalid tokens.
const MsgUnauthenticated = "Требуется вход"

// ErrInvalidToken is returned by Verifier for any malformed, expired or forged token.
var ErrInvalidToken = errors.New("auth: invalid token")

// Issuer issues access tokens.
type Issuer interface {
	Issue(userID uuid.UUID) (token string, err error)
}

// Verifier verifies access tokens.
type Verifier interface {
	Verify(token string) (userID uuid.UUID, err error)
}

// HashPassword returns the bcrypt hash of password.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CheckPassword reports whether password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

type ctxKey struct{}

// WithUserID returns a copy of ctx carrying the authenticated user id.
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// UserIDFromContext returns the authenticated user id set by the interceptor.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}
