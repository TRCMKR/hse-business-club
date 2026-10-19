package bff

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"

	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

func TestV1Register(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	const display = "Анна"

	resp, err := e.bff.V1Register(ctx, &hbcv1.V1RegisterRequest{Username: "Anna", Password: testPassword, DisplayName: display})
	wantOK(t, err)
	u := resp.GetUser()
	if resp.GetToken() != "token:"+u.GetId() {
		t.Fatalf("token = %q for user %s", resp.GetToken(), u.GetId())
	}
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
	if resp.GetToken() != "token:"+anna.ID.String() || resp.GetUser().GetId() != anna.ID.String() {
		t.Fatalf("resp = %v", resp)
	}

	for _, req := range []*hbcv1.V1LoginRequest{
		{Username: nameAnna, Password: "wrong-pass"},
		{Username: "nobody", Password: testPassword},
	} {
		_, err = e.bff.V1Login(ctx, req)
		wantStatus(t, err, codes.Unauthenticated, fmsgBadLogin)
	}
}
