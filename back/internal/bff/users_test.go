package bff

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
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

func TestV1GetMe(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)

	u, err := e.bff.V1GetMe(as(anna.ID), &hbcv1.V1GetMeRequest{})
	wantOK(t, err)
	if u.GetId() != anna.ID.String() || u.GetUsername() != nameAnna || u.GetCreatedAt().AsTime() != anna.CreatedAt {
		t.Fatalf("user = %v", u)
	}

	e.users.err = errors.New("pgx: connection refused")
	_, err = e.bff.V1GetMe(as(anna.ID), &hbcv1.V1GetMeRequest{})
	wantStatus(t, err, codes.Internal, msgInternal)
}

func TestV1UpdateMe(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)

	u, err := e.bff.V1UpdateMe(as(anna.ID), &hbcv1.V1UpdateMeRequest{
		DisplayName: "Анна Соколова", Headline: "Фаундер", About: "О себе", Skills: []string{"Продукт", "Финтех"}, Telegram: "@anna",
	})
	wantOK(t, err)
	if u.GetDisplayName() != "Анна Соколова" || u.GetHeadline() != "Фаундер" || u.GetAbout() != "О себе" ||
		strings.Join(u.GetSkills(), ",") != "Продукт,Финтех" || u.GetTelegram() != nameAnna {
		t.Fatalf("user = %v", u)
	}

	_, err = e.bff.V1UpdateMe(as(anna.ID), &hbcv1.V1UpdateMeRequest{})
	wantStatus(t, err, codes.InvalidArgument, fmsgDisplayName)
}

func TestV1GetUser(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris := e.user(t, nameAnna), e.user(t, nameBoris)

	u, err := e.bff.V1GetUser(as(boris.ID), &hbcv1.V1GetUserRequest{Id: anna.ID.String()})
	wantOK(t, err)
	if u.GetId() != anna.ID.String() || u.GetUsername() != nameAnna {
		t.Fatalf("user = %v", u)
	}

	e.calls.reset()
	_, err = e.bff.V1GetUser(as(boris.ID), &hbcv1.V1GetUserRequest{Id: "not-a-uuid"})
	wantStatus(t, err, codes.NotFound, msgUserNotFound)
	e.noCalls(t)

	_, err = e.bff.V1GetUser(as(boris.ID), &hbcv1.V1GetUserRequest{Id: "00000000-0000-4000-8000-000000000000"})
	wantStatus(t, err, codes.NotFound, msgUserNotFound)
}

func TestV1UploadAvatar(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)
	ctx := as(anna.ID)

	e.calls.reset()
	_, err := e.bff.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{})
	wantStatus(t, err, codes.InvalidArgument, msgAvatarEmpty)
	e.noCalls(t)

	_, err = e.bff.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{Data: notImage})
	wantStatus(t, err, codes.InvalidArgument, fmsgAvatarFormat)

	u, err := e.bff.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{Data: []byte("jpeg-1")})
	wantOK(t, err)
	first := u.GetAvatarUrl()
	if !strings.HasPrefix(first, "/v1/avatars/") || !strings.HasSuffix(first, ".jpg") {
		t.Fatalf("avatarUrl = %q", first)
	}
	u, err = e.bff.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{Data: []byte("jpeg-2")})
	wantOK(t, err)
	if u.GetAvatarUrl() == first {
		t.Fatal("avatarUrl must change on every upload")
	}
}

func TestV1DeleteAvatar(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)
	ctx := as(anna.ID)
	_, err := e.bff.V1UploadAvatar(ctx, &hbcv1.V1UploadAvatarRequest{Data: []byte("jpeg")})
	wantOK(t, err)

	for range 2 { // idempotent
		u, err := e.bff.V1DeleteAvatar(ctx, &hbcv1.V1DeleteAvatarRequest{})
		wantOK(t, err)
		if u.GetAvatarUrl() != "" || u.GetId() != anna.ID.String() {
			t.Fatalf("user = %v", u)
		}
	}
}

func TestV1GetAvatar(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)
	u, err := e.bff.V1UploadAvatar(as(anna.ID), &hbcv1.V1UploadAvatarRequest{Data: []byte("jpeg-bytes")})
	wantOK(t, err)
	file := strings.TrimPrefix(u.GetAvatarUrl(), avatarPathPrefix)

	stream := &headerStream{}
	ctx := grpc.NewContextWithServerTransportStream(context.Background(), stream) // public: no user
	body, err := e.bff.V1GetAvatar(ctx, &hbcv1.V1GetAvatarRequest{File: file})
	wantOK(t, err)
	if body.GetContentType() != jpegType || !bytes.Equal(body.GetData(), []byte("jpeg-bytes")) {
		t.Fatalf("body = %s %q", body.GetContentType(), body.GetData())
	}
	if got := stream.md.Get("cache-control"); len(got) != 1 || got[0] != "public, max-age=31536000, immutable" {
		t.Fatalf("cache-control = %v", got)
	}

	_, err = e.bff.V1GetAvatar(ctx, &hbcv1.V1GetAvatarRequest{File: "unknown.jpg"})
	wantStatus(t, err, codes.NotFound, fmsgAvatarNotFound)
}

// TestPrivateMethodsRequireUser checks the safety net behind the auth interceptor:
// every private RPC answers Unauthenticated without touching ports.
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
			wantStatus(t, call(), codes.Unauthenticated, "Требуется вход")
		})
	}
	e.noCalls(t)
}
