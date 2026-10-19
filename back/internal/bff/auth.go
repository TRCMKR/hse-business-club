package bff

import (
	"context"

	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// V1Register is not implemented yet (B4).
func (i *Implementation) V1Register(_ context.Context, _ *hbcv1.V1RegisterRequest) (*hbcv1.V1AuthResponse, error) {
	return nil, errNotImplemented
}

// V1Login is not implemented yet (B4).
func (i *Implementation) V1Login(_ context.Context, _ *hbcv1.V1LoginRequest) (*hbcv1.V1AuthResponse, error) {
	return nil, errNotImplemented
}
