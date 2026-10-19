package bff

import (
	"context"

	"google.golang.org/genproto/googleapis/api/httpbody"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/trcmkr/hse-business-club/back/internal/users"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// avatarCacheControl: the file name changes on every upload, so the file is immutable.
const avatarCacheControl = "public, max-age=31536000, immutable"

// V1GetMe returns the current user.
func (i *Implementation) V1GetMe(ctx context.Context, _ *hbcv1.V1GetMeRequest) (*hbcv1.User, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	u, err := i.Users.Get(ctx, me)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return userProto(u), nil
}

// V1UpdateMe replaces the editable profile fields of the current user.
func (i *Implementation) V1UpdateMe(ctx context.Context, req *hbcv1.V1UpdateMeRequest) (*hbcv1.User, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	u, err := i.Users.UpdateProfile(ctx, me, users.UpdateProfileParams{
		DisplayName: req.GetDisplayName(),
		Headline:    req.GetHeadline(),
		About:       req.GetAbout(),
		Skills:      req.GetSkills(),
		Telegram:    req.GetTelegram(),
	})
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return userProto(u), nil
}

// V1GetUser returns a public profile.
func (i *Implementation) V1GetUser(ctx context.Context, req *hbcv1.V1GetUserRequest) (*hbcv1.User, error) {
	if _, err := currentUser(ctx); err != nil {
		return nil, err
	}
	id, err := parsePathID(req.GetId(), msgUserNotFound)
	if err != nil {
		return nil, err
	}
	u, err := i.Users.Get(ctx, id)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return userProto(u), nil
}

// V1UploadAvatar sets or replaces the photo of the current user.
func (i *Implementation) V1UploadAvatar(ctx context.Context, req *hbcv1.V1UploadAvatarRequest) (*hbcv1.User, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.GetData()) == 0 {
		return nil, status.Error(codes.InvalidArgument, msgAvatarEmpty)
	}
	u, err := i.Users.SetAvatar(ctx, me, req.GetData())
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return userProto(u), nil
}

// V1DeleteAvatar removes the photo of the current user (idempotent).
func (i *Implementation) V1DeleteAvatar(ctx context.Context, _ *hbcv1.V1DeleteAvatarRequest) (*hbcv1.User, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	u, err := i.Users.RemoveAvatar(ctx, me)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return userProto(u), nil
}

// V1GetAvatar serves a photo file as raw bytes (public: the name is an unguessable uuid).
func (i *Implementation) V1GetAvatar(ctx context.Context, req *hbcv1.V1GetAvatarRequest) (*httpbody.HttpBody, error) {
	a, err := i.Users.OpenAvatar(ctx, req.GetFile())
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	if err := grpc.SetHeader(ctx, metadata.Pairs("cache-control", avatarCacheControl)); err != nil {
		return nil, toStatus(ctx, err)
	}
	return &httpbody.HttpBody{ContentType: a.ContentType, Data: a.Data}, nil
}
