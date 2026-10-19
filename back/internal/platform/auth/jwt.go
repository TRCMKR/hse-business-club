package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWT issues and verifies HS256 tokens with claims sub (user id), iat and exp.
type JWT struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

var (
	_ Issuer   = (*JWT)(nil)
	_ Verifier = (*JWT)(nil)
)

// NewJWT creates a JWT issuer/verifier.
func NewJWT(secret string, ttl time.Duration) *JWT {
	return &JWT{secret: []byte(secret), ttl: ttl, now: time.Now}
}

// WithClock returns a copy of j that uses now as the current time (for tests).
func (j *JWT) WithClock(now func() time.Time) *JWT {
	c := *j
	c.now = now
	return &c
}

// Issue implements Issuer.
func (j *JWT) Issue(userID uuid.UUID) (string, error) {
	now := j.now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign token: %w", err)
	}
	return token, nil
}

// Verify implements Verifier. Any problem yields an error wrapping ErrInvalidToken.
func (j *JWT) Verify(token string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil {
		return uuid.Nil, errors.Join(ErrInvalidToken, err)
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: bad subject", ErrInvalidToken)
	}
	return id, nil
}
