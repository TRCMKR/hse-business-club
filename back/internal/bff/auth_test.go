package bff

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// wantPair checks an auth response for u: access from the issuer, the refresh token
// from the users context, access_expires_in from AccessTTL.
func wantPair(t *testing.T, resp *hbcv1.V1AuthResponse, u *users.User, refresh string) {
	t.Helper()
	require.Equal(t, accessToken(u.ID), resp.GetAccessToken())
	require.Equal(t, refresh, resp.GetRefreshToken())
	require.Equal(t, u.ID.String(), resp.GetUser().GetId())
	require.Equal(t, int32(testAccessTTL/time.Second), resp.GetAccessExpiresIn())
}

func refreshToken(token string) *users.RefreshToken {
	return &users.RefreshToken{Token: token, ExpiresAt: baseTime.Add(time.Hour)}
}

func TestV1Register(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	const display = "Анна"

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		anna := newUser(nameAnna)
		anna.DisplayName = display
		e.users.EXPECT().Register(mock.Anything, users.RegisterParams{Username: "Anna", Password: testPassword, DisplayName: display}).
			Return(anna, nil).Once()
		e.users.EXPECT().IssueRefresh(mock.Anything, anna.ID).Return(refreshToken(testRefresh), nil).Once()
		e.expectIssue(anna.ID)

		resp, err := e.bff.V1Register(ctx, &hbcv1.V1RegisterRequest{Username: "Anna", Password: testPassword, DisplayName: display})
		wantOK(t, err)
		wantPair(t, resp, anna, testRefresh)
		u := resp.GetUser()
		if u.GetUsername() != nameAnna || u.GetDisplayName() != display || u.GetAvatarUrl() != "" || u.GetCreatedAt() == nil {
			t.Fatalf("user = %v", u)
		}
	})

	// Context errors pass through with their message; no tokens are issued.
	for _, tt := range []struct {
		name string
		err  error
		code codes.Code
		msg  string
	}{
		{"login taken", errOf(apperr.KindConflict, cmsgLoginTaken), codes.AlreadyExists, cmsgLoginTaken},
		{"bad username", apperr.Validation("username", cmsgUsername), codes.InvalidArgument, cmsgUsername},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.users.EXPECT().Register(mock.Anything, mock.Anything).Return(nil, tt.err).Once()

			_, err := e.bff.V1Register(ctx, &hbcv1.V1RegisterRequest{Username: "an", Password: testPassword})
			wantStatus(t, err, tt.code, tt.msg)
		})
	}
}

func TestV1RegisterIssuerFailure(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := newUser(nameAnna)
	e.users.EXPECT().Register(mock.Anything, mock.Anything).Return(anna, nil).Once()
	e.users.EXPECT().IssueRefresh(mock.Anything, anna.ID).Return(refreshToken(testRefresh), nil).Once()
	e.issuer.EXPECT().Issue(anna.ID).Return("", errors.New("jwt: boom")).Once()

	_, err := e.bff.V1Register(context.Background(), &hbcv1.V1RegisterRequest{Username: nameAnna, Password: testPassword, DisplayName: "Анна"})
	wantStatus(t, err, codes.Internal, msgInternal)
}

func TestV1Login(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		anna := newUser(nameAnna)
		e.users.EXPECT().Authenticate(mock.Anything, nameAnna, testPassword).Return(anna, nil).Once()
		e.users.EXPECT().IssueRefresh(mock.Anything, anna.ID).Return(refreshToken(testRefresh), nil).Once()
		e.expectIssue(anna.ID)

		resp, err := e.bff.V1Login(ctx, &hbcv1.V1LoginRequest{Username: nameAnna, Password: testPassword})
		wantOK(t, err)
		wantPair(t, resp, anna, testRefresh)
	})

	// Wrong password and unknown login: one message (the context decides, BFF passes it).
	t.Run("bad credentials", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().Authenticate(mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errOf(apperr.KindUnauthenticated, cmsgBadLogin)).Twice()

		for _, req := range []*hbcv1.V1LoginRequest{
			{Username: nameAnna, Password: "wrong-pass"},
			{Username: "nobody", Password: testPassword},
		} {
			_, err := e.bff.V1Login(ctx, req)
			wantStatus(t, err, codes.Unauthenticated, cmsgBadLogin)
		}
	})
}

func TestV1LoginRefreshFailure(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := newUser(nameAnna)
	e.users.EXPECT().Authenticate(mock.Anything, nameAnna, testPassword).Return(anna, nil).Once()
	e.users.EXPECT().IssueRefresh(mock.Anything, anna.ID).Return(nil, errors.New("db: boom")).Once()

	_, err := e.bff.V1Login(context.Background(), &hbcv1.V1LoginRequest{Username: nameAnna, Password: testPassword})
	wantStatus(t, err, codes.Internal, msgInternal)
}

func TestV1Refresh(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("rotation", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		anna := newUser(nameAnna)
		e.users.EXPECT().RotateRefresh(mock.Anything, testRefresh).Return(anna.ID, refreshToken("next"), nil).Once()
		e.users.EXPECT().Get(mock.Anything, anna.ID).Return(anna, nil).Once()
		e.expectIssue(anna.ID)

		next, err := e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: testRefresh})
		wantOK(t, err)
		wantPair(t, next, anna, "next")
		// Exactly one RotateRefresh and one Get: .Once() above, checked on cleanup.
	})

	// Unknown, expired, revoked or reused token: the context answers 401 (and revokes
	// the family on reuse); the BFF passes it through.
	t.Run("rejected by the context", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().RotateRefresh(mock.Anything, "unknown").
			Return(uuid.Nil, nil, errOf(apperr.KindUnauthenticated, msgSessionExpired)).Once()

		_, err := e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: "unknown"})
		wantStatus(t, err, codes.Unauthenticated, msgSessionExpired)
	})

	// Empty and oversized tokens are rejected without calling the context.
	t.Run("malformed", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		for _, token := range []string{"", strings.Repeat("x", maxRefreshTokenLen+1)} {
			_, err := e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: token})
			wantStatus(t, err, codes.Unauthenticated, msgSessionExpired)
		}
	})

	t.Run("deleted user", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		id := uuid.New()
		e.users.EXPECT().RotateRefresh(mock.Anything, testRefresh).Return(id, refreshToken("next"), nil).Once()
		e.users.EXPECT().Get(mock.Anything, id).Return(nil, errOf(apperr.KindNotFound, msgUserNotFound)).Once()

		_, err := e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: testRefresh})
		wantStatus(t, err, codes.Unauthenticated, msgSessionExpired)
	})

	t.Run("users failure", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		id := uuid.New()
		e.users.EXPECT().RotateRefresh(mock.Anything, testRefresh).Return(id, refreshToken("next"), nil).Once()
		e.users.EXPECT().Get(mock.Anything, id).Return(nil, errors.New("db: boom")).Once()

		_, err := e.bff.V1Refresh(ctx, &hbcv1.V1RefreshRequest{RefreshToken: testRefresh})
		wantStatus(t, err, codes.Internal, msgInternal)
	})
}

func TestV1Logout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// A known or unknown token goes to the context (it revokes the whole family and is
	// idempotent); an empty one is answered 200 without calling it.
	t.Run("revokes", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().RevokeRefresh(mock.Anything, testRefresh).Return(nil).Twice()
		e.users.EXPECT().RevokeRefresh(mock.Anything, "unknown").Return(nil).Once()

		for _, token := range []string{testRefresh, testRefresh, "unknown", ""} {
			resp, err := e.bff.V1Logout(ctx, &hbcv1.V1LogoutRequest{RefreshToken: token})
			wantOK(t, err)
			require.NotNil(t, resp)
		}
	})

	t.Run("users failure", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().RevokeRefresh(mock.Anything, testRefresh).Return(errors.New("db: boom")).Once()

		_, err := e.bff.V1Logout(ctx, &hbcv1.V1LogoutRequest{RefreshToken: testRefresh})
		wantStatus(t, err, codes.Internal, msgInternal)
	})
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
