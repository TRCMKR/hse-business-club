package bff

import (
	"context"
	"errors"
	"maps"
	"math"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// wantPair checks an auth response for the user id: access from the issuer, a refresh
// token, access_expires_in from AccessTTL.
func wantPair(t *testing.T, resp *hbcv1.V1AuthResponse, userID string) {
	t.Helper()
	if resp.GetAccessToken() != "token:"+userID || resp.GetUser().GetId() != userID {
		t.Fatalf("access token %q / user %v, want user %s", resp.GetAccessToken(), resp.GetUser(), userID)
	}
	if !strings.HasPrefix(resp.GetRefreshToken(), "refresh:") {
		t.Fatalf("refresh token = %q", resp.GetRefreshToken())
	}
	if resp.GetAccessExpiresIn() != int32(testAccessTTL/time.Second) {
		t.Fatalf("access_expires_in = %d", resp.GetAccessExpiresIn())
	}
}

func TestV1Register(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	const display = "Анна"

	resp, err := e.bff.V1Register(ctx, &hbcv1.V1RegisterRequest{Username: "Anna", Password: testPassword, DisplayName: display})
	wantOK(t, err)
	u := resp.GetUser()
	wantPair(t, resp, u.GetId())
	if u.GetUsername() != nameAnna || u.GetDisplayName() != display || u.GetAvatarUrl() != "" || u.GetCreatedAt() == nil {
		t.Fatalf("user = %v", u)
	}

	_, err = e.bff.V1Register(ctx, &hbcv1.V1RegisterRequest{Username: nameAnna, Password: testPassword, DisplayName: "Анна 2"})
	wantStatus(t, err, codes.AlreadyExists, fmsgLoginTaken)

	_, err = e.bff.V1Register(ctx, &hbcv1.V1RegisterRequest{Username: "an", Password: testPassword, DisplayName: "Ан"})
	wantStatus(t, err, codes.InvalidArgument, fmsgUsername)
}

func TestV1RegisterIssuerFailure(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.bff.Issuer = fakeIssuer{err: errors.New("jwt: boom")}

	_, err := e.bff.V1Register(context.Background(), &hbcv1.V1RegisterRequest{Username: nameAnna, Password: testPassword, DisplayName: "Анна"})
	wantStatus(t, err, codes.Internal, msgInternal)
}

func TestV1Login(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)
	ctx := context.Background()

	resp, err := e.bff.V1Login(ctx, &hbcv1.V1LoginRequest{Username: nameAnna, Password: testPassword})
	wantOK(t, err)
	wantPair(t, resp, anna.ID.String())

	for _, req := range []*hbcv1.V1LoginRequest{
		{Username: nameAnna, Password: "wrong-pass"},
		{Username: "nobody", Password: testPassword},
	} {
		_, err = e.bff.V1Login(ctx, req)
		wantStatus(t, err, codes.Unauthenticated, fmsgBadLogin)
	}
}

func TestV1LoginRefreshFailure(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.user(t, nameAnna)
	e.users.err = errors.New("db: boom")

	_, err := e.bff.V1Login(context.Background(), &hbcv1.V1LoginRequest{Username: nameAnna, Password: testPassword})
	wantStatus(t, err, codes.Internal, msgInternal)
}

func TestV1Refresh(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)
	ctx := context.Background()

	login, err := e.bff.V1Login(ctx, &hbcv1.V1LoginRequest{Username: nameAnna, Password: testPassword})
	wantOK(t, err)

	e.calls.reset()
	next, err := e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: login.GetRefreshToken()})
	wantOK(t, err)
	wantPair(t, next, anna.ID.String())
	if next.GetRefreshToken() == login.GetRefreshToken() {
		t.Fatal("refresh token was not rotated")
	}
	wantCalls(t, e.calls, map[string]int{"Users.RotateRefresh": 1, "Users.Get": 1})

	// Reuse of the rotated token: 401 and the whole family is revoked.
	_, err = e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: login.GetRefreshToken()})
	wantStatus(t, err, codes.Unauthenticated, msgSessionExpired)
	_, err = e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: next.GetRefreshToken()})
	wantStatus(t, err, codes.Unauthenticated, msgSessionExpired)

	for _, token := range []string{"", "unknown", strings.Repeat("x", maxRefreshTokenLen+1)} {
		_, err = e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: token})
		wantStatus(t, err, codes.Unauthenticated, msgSessionExpired)
	}
}

func TestV1RefreshDeletedUser(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)
	ctx := context.Background()
	login, err := e.bff.V1Login(ctx, &hbcv1.V1LoginRequest{Username: nameAnna, Password: testPassword})
	wantOK(t, err)
	delete(e.users.byID, anna.ID)

	_, err = e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: login.GetRefreshToken()})
	wantStatus(t, err, codes.Unauthenticated, msgSessionExpired)
}

func TestV1Logout(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.user(t, nameAnna)
	ctx := context.Background()
	login, err := e.bff.V1Login(ctx, &hbcv1.V1LoginRequest{Username: nameAnna, Password: testPassword})
	wantOK(t, err)

	for range 2 { // idempotent
		_, err = e.bff.V1Logout(ctx, &hbcv1.V1LogoutRequest{RefreshToken: login.GetRefreshToken()})
		wantOK(t, err)
	}
	_, err = e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: login.GetRefreshToken()})
	wantStatus(t, err, codes.Unauthenticated, msgSessionExpired)

	e.calls.reset()
	for _, token := range []string{"", "unknown"} {
		_, err = e.bff.V1Logout(ctx, &hbcv1.V1LogoutRequest{RefreshToken: token})
		wantOK(t, err)
	}
	wantCalls(t, e.calls, map[string]int{"Users.RevokeRefresh": 1})

	e.users.err = errors.New("db: boom")
	_, err = e.bff.V1Logout(ctx, &hbcv1.V1LogoutRequest{RefreshToken: "refresh:1"})
	wantStatus(t, err, codes.Internal, msgInternal)
}

func TestExpiresIn(t *testing.T) {
	t.Parallel()
	for d, want := range map[time.Duration]int32{
		15 * time.Minute:                             900,
		1500 * time.Millisecond:                      1,
		-time.Second:                                 0,
		time.Duration(math.MaxInt64):                 math.MaxInt32,
		time.Duration(math.MaxInt32+1) * time.Second: math.MaxInt32,
	} {
		if got := expiresIn(d); got != want {
			t.Errorf("expiresIn(%s) = %d, want %d", d, got, want)
		}
	}
}

func wantCalls(t *testing.T, c *calls, want map[string]int) {
	t.Helper()
	if got := c.snapshot(); !maps.Equal(got, want) {
		t.Fatalf("port calls = %v, want %v", got, want)
	}
}
