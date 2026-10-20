package bff

import (
	"context"
	"math"
	"time"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// V1Register creates an account and signs the user in.
func (i *Implementation) V1Register(ctx context.Context, req *hbcv1.V1RegisterRequest) (*hbcv1.V1AuthResponse, error) {
	u, err := i.Users.Register(ctx, users.RegisterParams{
		Username:    req.GetUsername(),
		Password:    req.GetPassword(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return i.signIn(ctx, u)
}

// V1Login checks credentials and issues a token pair.
func (i *Implementation) V1Login(ctx context.Context, req *hbcv1.V1LoginRequest) (*hbcv1.V1AuthResponse, error) {
	u, err := i.Users.Authenticate(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return i.signIn(ctx, u)
}

// V1Refresh exchanges a refresh token for a new pair (rotation).
func (i *Implementation) V1Refresh(ctx context.Context, req *hbcv1.V1RefreshRequest) (*hbcv1.V1AuthResponse, error) {
	token, err := refreshTokenParam(req.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	userID, next, err := i.Users.RotateRefresh(ctx, token)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	u, err := i.Users.Get(ctx, userID)
	if apperr.KindOf(err) == apperr.KindNotFound {
		return nil, sessionExpired()
	}
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return i.authResponse(ctx, u, next)
}

// V1Logout revokes the refresh token (and its sign-in). Idempotent: an empty, unknown
// or already revoked token is not an error.
func (i *Implementation) V1Logout(ctx context.Context, req *hbcv1.V1LogoutRequest) (*hbcv1.V1LogoutResponse, error) {
	if token, err := refreshTokenParam(req.GetRefreshToken()); err == nil {
		if err := i.Users.RevokeRefresh(ctx, token); err != nil {
			return nil, toStatus(ctx, err)
		}
	}
	return &hbcv1.V1LogoutResponse{}, nil
}

// signIn starts a new sign-in: a new refresh token family plus an access token.
func (i *Implementation) signIn(ctx context.Context, u *users.User) (*hbcv1.V1AuthResponse, error) {
	refresh, err := i.Users.IssueRefresh(ctx, u.ID)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return i.authResponse(ctx, u, refresh)
}

func (i *Implementation) authResponse(ctx context.Context, u *users.User, refresh *users.RefreshToken) (*hbcv1.V1AuthResponse, error) {
	access, err := i.Issuer.Issue(u.ID)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return &hbcv1.V1AuthResponse{
		AccessToken:     access,
		RefreshToken:    refresh.Token,
		User:            userProto(u),
		AccessExpiresIn: expiresIn(i.AccessTTL),
	}, nil
}

// expiresIn is d in whole seconds, clamped to the int32 of the contract.
func expiresIn(d time.Duration) int32 {
	sec := int64(d / time.Second)
	if sec > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(max(sec, 0)) //nolint:gosec // clamped above
}
