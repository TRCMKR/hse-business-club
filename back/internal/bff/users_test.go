package bff

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

const (
	avatarKey    = "0b6f5a3e-1c2d-4e5f-8a9b-0c1d2e3f4a5b.jpg"
	unknownJPG   = "unknown.jpg"
	skillProduct = "Продукт"
)

// headerStream records headers set by grpc.SetHeader outside of a real server.
type headerStream struct{ md metadata.MD }

func (s *headerStream) Method() string { return hbcv1.HseBusinessClub_V1GetAvatar_FullMethodName }
func (s *headerStream) SetHeader(md metadata.MD) error {
	s.md = metadata.Join(s.md, md)
	return nil
}
func (s *headerStream) SendHeader(md metadata.MD) error { return s.SetHeader(md) }
func (s *headerStream) SetTrailer(metadata.MD) error    { return nil }

func withAvatar(u *users.User, key string) *users.User {
	c := *u
	c.AvatarKey = key
	return &c
}

func TestV1GetMe(t *testing.T) {
	t.Parallel()
	anna := newUser(nameAnna)
	anna.Headline, anna.About, anna.Skills, anna.Telegram = "Фаундер", "О себе", []string{skillProduct}, nameAnna
	anna = withAvatar(anna, avatarKey)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().Get(mock.Anything, anna.ID).Return(anna, nil).Once()

		u, err := e.bff.V1GetMe(as(anna.ID), &hbcv1.V1GetMeRequest{})
		wantOK(t, err)
		if u.GetId() != anna.ID.String() || u.GetUsername() != nameAnna || u.GetDisplayName() != anna.DisplayName ||
			u.GetHeadline() != anna.Headline || u.GetAbout() != anna.About || strings.Join(u.GetSkills(), ",") != skillProduct ||
			u.GetTelegram() != nameAnna || u.GetAvatarUrl() != avatarPathPrefix+avatarKey || u.GetCreatedAt().AsTime() != anna.CreatedAt {
			t.Fatalf("user = %v", u)
		}
	})

	t.Run(caseInternal, func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().Get(mock.Anything, anna.ID).Return(nil, errors.New("pgx: connection refused")).Once()

		_, err := e.bff.V1GetMe(as(anna.ID), &hbcv1.V1GetMeRequest{})
		wantStatus(t, err, codes.Internal, msgInternal)
	})
}

func TestV1UpdateMe(t *testing.T) {
	t.Parallel()
	anna := newUser(nameAnna)
	req := &hbcv1.V1UpdateMeRequest{
		DisplayName: "Анна Соколова", Headline: "Фаундер", About: "О себе", Skills: []string{skillProduct, "Финтех"}, Telegram: "@anna",
	}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		updated := *anna
		updated.DisplayName, updated.Headline, updated.About = req.GetDisplayName(), req.GetHeadline(), req.GetAbout()
		updated.Skills, updated.Telegram = req.GetSkills(), nameAnna // the context normalizes "@anna"
		e.users.EXPECT().UpdateProfile(mock.Anything, anna.ID, users.UpdateProfileParams{
			DisplayName: req.GetDisplayName(), Headline: req.GetHeadline(), About: req.GetAbout(),
			Skills: req.GetSkills(), Telegram: req.GetTelegram(),
		}).Return(&updated, nil).Once()

		u, err := e.bff.V1UpdateMe(as(anna.ID), req)
		wantOK(t, err)
		if u.GetDisplayName() != req.GetDisplayName() || u.GetHeadline() != req.GetHeadline() || u.GetAbout() != req.GetAbout() ||
			strings.Join(u.GetSkills(), ",") != "Продукт,Финтех" || u.GetTelegram() != nameAnna {
			t.Fatalf("user = %v", u)
		}
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().UpdateProfile(mock.Anything, anna.ID, users.UpdateProfileParams{}).
			Return(nil, apperr.Validation("display_name", cmsgDisplayName)).Once()

		_, err := e.bff.V1UpdateMe(as(anna.ID), &hbcv1.V1UpdateMeRequest{})
		wantStatus(t, err, codes.InvalidArgument, cmsgDisplayName)
	})
}

func TestV1GetUser(t *testing.T) {
	t.Parallel()
	anna, boris := newUser(nameAnna), newUser(nameBoris)

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().Get(mock.Anything, anna.ID).Return(anna, nil).Once()

		u, err := e.bff.V1GetUser(as(boris.ID), &hbcv1.V1GetUserRequest{Id: anna.ID.String()})
		wantOK(t, err)
		require.Equal(t, anna.ID.String(), u.GetId())
		require.Equal(t, nameAnna, u.GetUsername())
		require.Empty(t, u.GetAvatarUrl())
	})

	t.Run("bad id", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t) // no expectations: ports are not called
		_, err := e.bff.V1GetUser(as(boris.ID), &hbcv1.V1GetUserRequest{Id: "not-a-uuid"})
		wantStatus(t, err, codes.NotFound, msgUserNotFound)
	})

	t.Run("unknown id", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().Get(mock.Anything, anna.ID).Return(nil, errOf(apperr.KindNotFound, msgUserNotFound)).Once()

		_, err := e.bff.V1GetUser(as(boris.ID), &hbcv1.V1GetUserRequest{Id: anna.ID.String()})
		wantStatus(t, err, codes.NotFound, msgUserNotFound)
	})
}

func TestV1UploadAvatar(t *testing.T) {
	t.Parallel()
	anna := newUser(nameAnna)
	ctx := as(anna.ID)

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t) // no expectations: ports are not called
		_, err := e.bff.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{})
		wantStatus(t, err, codes.InvalidArgument, msgAvatarEmpty)
	})

	t.Run("not an image", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().SetAvatar(mock.Anything, anna.ID, []byte("not an image")).
			Return(nil, apperr.Validation("avatar", cmsgAvatarFormat)).Once()

		_, err := e.bff.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{Data: []byte("not an image")})
		wantStatus(t, err, codes.InvalidArgument, cmsgAvatarFormat)
	})

	// The bytes go to the context as is; avatarUrl is built from the new key, so it
	// changes on every upload.
	t.Run("upload and replace", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		const second = "1c7a6b4f-2d3e-4f5a-9b8c-1d2e3f4a5b6c.jpg"
		e.users.EXPECT().SetAvatar(mock.Anything, anna.ID, []byte("jpeg-1")).Return(withAvatar(anna, avatarKey), nil).Once()
		e.users.EXPECT().SetAvatar(mock.Anything, anna.ID, []byte("jpeg-2")).Return(withAvatar(anna, second), nil).Once()

		u, err := e.bff.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{Data: []byte("jpeg-1")})
		wantOK(t, err)
		require.Equal(t, avatarPathPrefix+avatarKey, u.GetAvatarUrl())
		u, err = e.bff.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{Data: []byte("jpeg-2")})
		wantOK(t, err)
		require.Equal(t, avatarPathPrefix+second, u.GetAvatarUrl())
	})
}

func TestV1DeleteAvatar(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := newUser(nameAnna)
	e.users.EXPECT().RemoveAvatar(mock.Anything, anna.ID).Return(anna, nil).Twice()

	for range 2 { // idempotent: the context answers the same, the BFF passes it
		u, err := e.bff.V1DeleteAvatar(as(anna.ID), &hbcv1.V1DeleteAvatarRequest{})
		wantOK(t, err)
		if u.GetAvatarUrl() != "" || u.GetId() != anna.ID.String() {
			t.Fatalf("user = %v", u)
		}
	}
}

func TestV1DeleteAvatarInternal(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := newUser(nameAnna)
	e.users.EXPECT().RemoveAvatar(mock.Anything, anna.ID).Return(nil, errors.New("s3: timeout")).Once()

	_, err := e.bff.V1DeleteAvatar(as(anna.ID), &hbcv1.V1DeleteAvatarRequest{})
	wantStatus(t, err, codes.Internal, msgInternal)
}

func TestV1GetAvatar(t *testing.T) {
	t.Parallel()
	data := []byte("jpeg-bytes")

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().OpenAvatar(mock.Anything, avatarKey).Return(&users.Avatar{ContentType: jpegType, Data: data}, nil).Once()

		stream := &headerStream{}
		ctx := grpc.NewContextWithServerTransportStream(context.Background(), stream) // public: no user
		body, err := e.bff.V1GetAvatar(ctx, &hbcv1.V1GetAvatarRequest{File: avatarKey})
		wantOK(t, err)
		if body.GetContentType() != jpegType || !bytes.Equal(body.GetData(), data) {
			t.Fatalf("body = %s %q", body.GetContentType(), body.GetData())
		}
		require.Equal(t, []string{avatarCacheControl}, stream.md.Get("cache-control"))
		require.Equal(t, "public, max-age=31536000, immutable", avatarCacheControl)
	})

	t.Run("unknown file", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().OpenAvatar(mock.Anything, unknownJPG).Return(nil, errOf(apperr.KindNotFound, cmsgAvatarNotFound)).Once()

		stream := &headerStream{}
		ctx := grpc.NewContextWithServerTransportStream(context.Background(), stream)
		_, err := e.bff.V1GetAvatar(ctx, &hbcv1.V1GetAvatarRequest{File: unknownJPG})
		wantStatus(t, err, codes.NotFound, cmsgAvatarNotFound)
		require.Empty(t, stream.md.Get("cache-control")) // errors are not cached as immutable
	})

	// Without a transport stream grpc.SetHeader fails: the error is not swallowed.
	t.Run("no stream", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.users.EXPECT().OpenAvatar(mock.Anything, avatarKey).Return(&users.Avatar{ContentType: jpegType, Data: data}, nil).Once()

		_, err := e.bff.V1GetAvatar(context.Background(), &hbcv1.V1GetAvatarRequest{File: avatarKey})
		require.Error(t, err)
	})
}

// TestPrivateMethodsRequireUser checks the safety net behind the auth interceptor:
// every private RPC answers Unauthenticated without touching ports (the mocks have
// no expectations, so any port call fails the test).
func TestPrivateMethodsRequireUser(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	b := e.bff
	id := "11111111-1111-4111-8111-111111111111"

	calls := map[string]func() error{
		"V1GetMe":    func() error { _, err := b.V1GetMe(ctx, &hbcv1.V1GetMeRequest{}); return err },
		"V1UpdateMe": func() error { _, err := b.V1UpdateMe(ctx, &hbcv1.V1UpdateMeRequest{}); return err },
		"V1GetUser":  func() error { _, err := b.V1GetUser(ctx, &hbcv1.V1GetUserRequest{Id: id}); return err },
		"V1UploadAvatar": func() error {
			_, err := b.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{Data: []byte("x")})
			return err
		},
		"V1DeleteAvatar": func() error { _, err := b.V1DeleteAvatar(ctx, &hbcv1.V1DeleteAvatarRequest{}); return err },
		"V1ListListings": func() error { _, err := b.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{}); return err },
		"V1CreateListing": func() error {
			_, err := b.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{Kind: hbcv1.ListingKind_LISTING_KIND_VACANCY})
			return err
		},
		"V1GetListing":     func() error { _, err := b.V1GetListing(ctx, &hbcv1.V1GetListingRequest{Id: id}); return err },
		"V1UpdateListing":  func() error { _, err := b.V1UpdateListing(ctx, &hbcv1.V1UpdateListingRequest{Id: id}); return err },
		"V1CloseListing":   func() error { _, err := b.V1CloseListing(ctx, &hbcv1.V1CloseListingRequest{Id: id}); return err },
		"V1ListMyListings": func() error { _, err := b.V1ListMyListings(ctx, &hbcv1.V1ListMyListingsRequest{}); return err },
		"V1CreateApplication": func() error {
			_, err := b.V1CreateApplication(ctx, &hbcv1.V1CreateApplicationRequest{ListingId: id, Message: "x"})
			return err
		},
		"V1ListListingApplications": func() error {
			_, err := b.V1ListListingApplications(ctx, &hbcv1.V1ListListingApplicationsRequest{ListingId: id})
			return err
		},
		"V1AcceptApplication":  func() error { _, err := b.V1AcceptApplication(ctx, &hbcv1.V1ApplicationIDRequest{Id: id}); return err },
		"V1DeclineApplication": func() error { _, err := b.V1DeclineApplication(ctx, &hbcv1.V1ApplicationIDRequest{Id: id}); return err },
		"V1WithdrawApplication": func() error {
			_, err := b.V1WithdrawApplication(ctx, &hbcv1.V1ApplicationIDRequest{Id: id})
			return err
		},
		"V1ListSentApplications": func() error {
			_, err := b.V1ListSentApplications(ctx, &hbcv1.V1ListSentApplicationsRequest{})
			return err
		},
		"V1ListReceivedApplications": func() error {
			_, err := b.V1ListReceivedApplications(ctx, &hbcv1.V1ListReceivedApplicationsRequest{})
			return err
		},
	}
	if got, want := len(calls)+len(publicMethods), len(hbcv1.HseBusinessClub_ServiceDesc.Methods); got != want {
		t.Fatalf("private %d + public %d methods != %d RPCs in the service", len(calls), len(publicMethods), want)
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			wantStatus(t, call(), codes.Unauthenticated, auth.MsgUnauthenticated)
		})
	}
	require.Equal(t, "Требуется вход", auth.MsgUnauthenticated)
}
