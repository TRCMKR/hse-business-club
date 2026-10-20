package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
)

const secret = "test-secret-test-secret-test-secret"

func TestPassword(t *testing.T) {
	t.Parallel()

	h, err := auth.HashPassword("demo1234")
	if err != nil {
		t.Fatal(err)
	}
	if !auth.CheckPassword(h, "demo1234") {
		t.Fatal("correct password rejected")
	}
	if auth.CheckPassword(h, "demo12345") {
		t.Fatal("wrong password accepted")
	}
}

func TestJWTIssueVerify(t *testing.T) {
	t.Parallel()

	j := auth.NewJWT(secret, time.Hour)
	id := uuid.New()
	tok, err := j.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := j.Verify(tok)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got != id {
		t.Fatalf("Verify() = %s, want %s", got, id)
	}
	if j.TTL() != time.Hour {
		t.Fatalf("TTL() = %s, want 1h", j.TTL())
	}
}

func TestJWTExpired(t *testing.T) {
	t.Parallel()

	past := time.Now().Add(-2 * time.Hour)
	tok, err := auth.NewJWT(secret, time.Hour).WithClock(func() time.Time { return past }).Issue(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.NewJWT(secret, time.Hour).Verify(tok); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expired token: err = %v, want ErrInvalidToken", err)
	}
}

func TestJWTForeignSecretAndGarbage(t *testing.T) {
	t.Parallel()

	tok, err := auth.NewJWT("another-secret-another-secret-xx", time.Hour).Issue(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	j := auth.NewJWT(secret, time.Hour)
	for name, tk := range map[string]string{"foreign secret": tok, "garbage": "not.a.jwt", "empty": ""} {
		if _, err := j.Verify(tk); !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("%s: err = %v, want ErrInvalidToken", name, err)
		}
	}
}

func TestUnaryInterceptor(t *testing.T) {
	t.Parallel()

	const (
		publicMethod  = "/hbc.v1.HseBusinessClub/V1Ping"
		privateMethod = "/hbc.v1.HseBusinessClub/V1GetMe"
	)
	j := auth.NewJWT(secret, time.Hour)
	id := uuid.New()
	tok, err := j.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	icpt := auth.UnaryInterceptor(j, map[string]bool{publicMethod: true})

	call := func(method, authorization string) (uuid.UUID, bool, error) {
		ctx := context.Background()
		if authorization != "" {
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", authorization))
		}
		var (
			gotID uuid.UUID
			gotOK bool
		)
		_, err := icpt(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
			gotID, gotOK = auth.UserIDFromContext(ctx)
			return "ok", nil
		})
		return gotID, gotOK, err
	}

	assertUnauthenticated := func(t *testing.T, err error) {
		t.Helper()
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.Unauthenticated || st.Message() != "Требуется вход" {
			t.Fatalf("err = %v, want Unauthenticated «Требуется вход»", err)
		}
	}

	t.Run("public without token", func(t *testing.T) {
		t.Parallel()
		_, ok, err := call(publicMethod, "")
		if err != nil || ok {
			t.Fatalf("err = %v, ok = %v", err, ok)
		}
	})
	t.Run("public with bad token", func(t *testing.T) {
		t.Parallel()
		_, ok, err := call(publicMethod, "Bearer garbage")
		if err != nil || ok {
			t.Fatalf("err = %v, ok = %v", err, ok)
		}
	})
	t.Run("public with good token", func(t *testing.T) {
		t.Parallel()
		got, ok, err := call(publicMethod, "Bearer "+tok)
		if err != nil || !ok || got != id {
			t.Fatalf("err = %v, ok = %v, id = %s", err, ok, got)
		}
	})
	t.Run("private without token", func(t *testing.T) {
		t.Parallel()
		_, _, err := call(privateMethod, "")
		assertUnauthenticated(t, err)
	})
	t.Run("private with bad token", func(t *testing.T) {
		t.Parallel()
		_, _, err := call(privateMethod, "Bearer garbage")
		assertUnauthenticated(t, err)
	})
	t.Run("private with non-bearer scheme", func(t *testing.T) {
		t.Parallel()
		_, _, err := call(privateMethod, "Basic "+tok)
		assertUnauthenticated(t, err)
	})
	t.Run("private with good token", func(t *testing.T) {
		t.Parallel()
		got, ok, err := call(privateMethod, "bearer "+tok)
		if err != nil || !ok || got != id {
			t.Fatalf("err = %v, ok = %v, id = %s", err, ok, got)
		}
	})
}
