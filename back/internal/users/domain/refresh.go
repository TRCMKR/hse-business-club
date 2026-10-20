package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RefreshTokenBytes is the entropy of a refresh token (SPEC §4.2).
const RefreshTokenBytes = 32

// RefreshTokenLen is the length of an encoded refresh token: base64url without padding.
var RefreshTokenLen = base64.RawURLEncoding.EncodedLen(RefreshTokenBytes)

// StoredRefreshToken is a row of users.refresh_tokens. The token itself is never
// stored, only its SHA-256.
type StoredRefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	FamilyID  uuid.UUID
	Hash      []byte
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time // nil = active
}

// NewRefreshToken returns a random opaque token and its hash.
func NewRefreshToken() (token string, hash []byte, err error) {
	b := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("refresh token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken is the stored form of a token: SHA-256 of its string. A token is
// 256 random bits, so a plain fast hash is enough (no salt, no KDF).
func HashRefreshToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
