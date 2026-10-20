//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	msgLoginRequired  = "Требуется вход"
	msgSessionExpired = "Сессия истекла"
)

// SPEC §2.3, §4.2: access expires → 401 «Требуется вход» → V1Refresh gives a new pair
// (rotation); reuse of a rotated refresh token revokes the family; logout revokes the
// sign-in. Refresh and logout need no Authorization.
func TestRefreshTokens(t *testing.T) {
	e := newEnv(t, withAccessTTL(time.Second))
	unauth := http.StatusUnauthorized
	e.register("anna", "Анна")

	out := e.ok(http.MethodPost, "/v1/auth/login", "", J{"username": "anna", "password": password})
	first := expectPair(t, "login", out)
	if got := num(out["accessExpiresIn"]); got != 1 {
		t.Fatalf("accessExpiresIn = %d, want 1 (ACCESS_TTL=1s)", got)
	}
	e.ok(http.MethodGet, "/v1/me", first.access, nil)

	// JWT exp has a one-second resolution: after 2 s the access token has expired.
	time.Sleep(2 * time.Second)
	e.fails(http.MethodGet, "/v1/me", first.access, nil, unauth, 16, msgLoginRequired)

	out = e.ok(http.MethodPost, "/v1/auth/refresh", "", J{"refreshToken": first.refresh})
	second := expectPair(t, "refresh", out)
	if second.refresh == first.refresh || second.access == first.access {
		t.Fatal("refresh must return a new pair")
	}
	if u := obj(t, out, "user"); u["username"] != "anna" {
		t.Fatalf("refresh user = %v", u)
	}
	expectUser(t, obj(t, out, "user"))
	e.ok(http.MethodGet, "/v1/me", second.access, nil)

	// Reuse of the rotated token: 401, and the family is revoked (the new token too).
	e.fails(http.MethodPost, "/v1/auth/refresh", "", J{"refreshToken": first.refresh}, unauth, 16, msgSessionExpired)
	e.fails(http.MethodPost, "/v1/auth/refresh", "", J{"refreshToken": second.refresh}, unauth, 16, msgSessionExpired)

	// Bad input.
	for _, body := range []any{J{}, J{"refreshToken": ""}, J{"refreshToken": "unknown"}} {
		e.fails(http.MethodPost, "/v1/auth/refresh", "", body, unauth, 16, msgSessionExpired)
	}

	// Logout revokes the sign-in; it is idempotent and public.
	p := e.loginPair("anna")
	other := e.loginPair("anna") // another device
	for range 2 {
		if out := e.ok(http.MethodPost, "/v1/auth/logout", "", J{"refreshToken": p.refresh}); len(out) != 0 {
			t.Fatalf("logout = %v, want {}", out)
		}
	}
	e.fails(http.MethodPost, "/v1/auth/refresh", "", J{"refreshToken": p.refresh}, unauth, 16, msgSessionExpired)
	for _, body := range []any{J{}, J{"refreshToken": "unknown"}} {
		e.ok(http.MethodPost, "/v1/auth/logout", "", body)
	}
	e.ok(http.MethodPost, "/v1/auth/refresh", "", J{"refreshToken": other.refresh})

	// Only SHA-256 hashes are stored.
	var plain int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM users.refresh_tokens t
		WHERE t::text LIKE '%' || $1 || '%' OR t::text LIKE '%' || $2 || '%' OR length(token_hash) <> 32`,
		first.refresh, other.refresh).Scan(&plain)
	if err != nil || plain != 0 {
		t.Fatalf("plain tokens in the table: %d, %v", plain, err)
	}
	// Tokens never reach the logs.
	logs := e.logs.String()
	for _, tok := range []string{first.refresh, second.refresh, p.refresh, first.access} {
		if strings.Contains(logs, tok) {
			t.Fatal("a token is in the logs")
		}
	}
}
