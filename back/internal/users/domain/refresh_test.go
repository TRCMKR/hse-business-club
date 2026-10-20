package domain_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
)

func TestNewRefreshToken(t *testing.T) {
	t.Parallel()

	tok, hash, err := domain.NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != domain.RefreshTokenLen || domain.RefreshTokenLen != 43 {
		t.Fatalf("token length = %d, want 43", len(tok))
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil || len(raw) != domain.RefreshTokenBytes {
		t.Fatalf("token %q is not base64url of 32 bytes: %v", tok, err)
	}
	if !bytes.Equal(hash, domain.HashRefreshToken(tok)) || len(hash) != 32 {
		t.Fatalf("hash mismatch")
	}
	other, _, err := domain.NewRefreshToken()
	if err != nil || other == tok {
		t.Fatalf("tokens must be random: %q %q %v", tok, other, err)
	}
}
