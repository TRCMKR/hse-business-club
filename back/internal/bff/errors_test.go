package bff

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

func TestToStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
		wantMsg  string
	}{
		{"validation", apperr.Validation("title", "Название: от 3 до 120 символов"), codes.InvalidArgument, "Название: от 3 до 120 символов"},
		{"unauthenticated", apperr.New(apperr.KindUnauthenticated, "Неверный логин или пароль"), codes.Unauthenticated, "Неверный логин или пароль"},
		{"forbidden", apperr.New(apperr.KindForbidden, "Нет доступа"), codes.PermissionDenied, "Нет доступа"},
		{"not found", apperr.New(apperr.KindNotFound, "Публикация не найдена"), codes.NotFound, "Публикация не найдена"},
		{"conflict", apperr.New(apperr.KindConflict, "Вы уже откликались на эту публикацию"), codes.AlreadyExists, "Вы уже откликались на эту публикацию"},
		{"precondition", apperr.New(apperr.KindPrecondition, "Публикация закрыта"), codes.FailedPrecondition, "Публикация закрыта"},
		{"wrapped apperr", fmt.Errorf("svc: %w", apperr.New(apperr.KindNotFound, "Нет")), codes.NotFound, "Нет"},
		{"unknown kind", apperr.Wrap(apperr.KindUnknown, "secret detail", errors.New("db")), codes.Internal, msgInternal},
		{"plain error", errors.New("pgx: connection refused"), codes.Internal, msgInternal},
		{"status passthrough", status.Error(codes.Unauthenticated, "Требуется вход"), codes.Unauthenticated, "Требуется вход"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			st := status.Convert(toStatus(context.Background(), tt.err))
			if st.Code() != tt.wantCode || st.Message() != tt.wantMsg {
				t.Fatalf("got %s %q, want %s %q", st.Code(), st.Message(), tt.wantCode, tt.wantMsg)
			}
		})
	}
	if toStatus(context.Background(), nil) != nil {
		t.Fatal("nil must stay nil")
	}
}
