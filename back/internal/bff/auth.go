package bff

import (
	"context"

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
	return i.authResponse(ctx, u)
}

// V1Login checks credentials and issues a token.
func (i *Implementation) V1Login(ctx context.Context, req *hbcv1.V1LoginRequest) (*hbcv1.V1AuthResponse, error) {
	u, err := i.Users.Authenticate(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return i.authResponse(ctx, u)
}

func (i *Implementation) authResponse(ctx context.Context, u *users.User) (*hbcv1.V1AuthResponse, error) {
	token, err := i.Issuer.Issue(u.ID)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return &hbcv1.V1AuthResponse{Token: token, User: userProto(u)}, nil
}
