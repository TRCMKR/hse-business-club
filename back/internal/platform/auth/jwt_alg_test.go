package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
)

// Only HS256 is accepted: "none", RS256 and other HMAC variants are rejected even
// when the claims are valid.
func TestJWTRejectsOtherAlgorithms(t *testing.T) {
	const secret = "test-secret"
	v := auth.NewJWT(secret, time.Hour)
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   uuid.NewString(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	sign := func(t *testing.T, m jwt.SigningMethod, key any) string {
		t.Helper()
		tok, err := jwt.NewWithClaims(m, claims).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	cases := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{"none", func(t *testing.T) string { return sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType) }},
		{"RS256", func(t *testing.T) string { return sign(t, jwt.SigningMethodRS256, rsaKey) }},
		{"HS512 with the same secret", func(t *testing.T) string { return sign(t, jwt.SigningMethodHS512, []byte(secret)) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := v.Verify(c.token(t)); !errors.Is(err, auth.ErrInvalidToken) {
				t.Fatalf("Verify() error = %v, want ErrInvalidToken", err)
			}
		})
	}

	// Control: the same claims signed with HS256 and the secret pass.
	if _, err := v.Verify(sign(t, jwt.SigningMethodHS256, []byte(secret))); err != nil {
		t.Fatalf("HS256 control: %v", err)
	}
}
