package bff

import (
	"context"

	"google.golang.org/genproto/googleapis/api/httpbody"

	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// V1GetMe is not implemented yet (B4).
func (i *Implementation) V1GetMe(_ context.Context, _ *hbcv1.V1GetMeRequest) (*hbcv1.User, error) {
	return nil, errNotImplemented
}

// V1UpdateMe is not implemented yet (B4).
func (i *Implementation) V1UpdateMe(_ context.Context, _ *hbcv1.V1UpdateMeRequest) (*hbcv1.User, error) {
	return nil, errNotImplemented
}

// V1GetUser is not implemented yet (B4).
func (i *Implementation) V1GetUser(_ context.Context, _ *hbcv1.V1GetUserRequest) (*hbcv1.User, error) {
	return nil, errNotImplemented
}

// V1UploadAvatar is not implemented yet (B4).
func (i *Implementation) V1UploadAvatar(_ context.Context, _ *hbcv1.V1UploadAvatarRequest) (*hbcv1.User, error) {
	return nil, errNotImplemented
}

// V1DeleteAvatar is not implemented yet (B4).
func (i *Implementation) V1DeleteAvatar(_ context.Context, _ *hbcv1.V1DeleteAvatarRequest) (*hbcv1.User, error) {
	return nil, errNotImplemented
}

// V1GetAvatar is not implemented yet (B4).
func (i *Implementation) V1GetAvatar(_ context.Context, _ *hbcv1.V1GetAvatarRequest) (*httpbody.HttpBody, error) {
	return nil, errNotImplemented
}
