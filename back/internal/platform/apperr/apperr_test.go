package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

func TestKindOf(t *testing.T) {
	t.Parallel()

	cause := errors.New("db down")
	tests := []struct {
		name string
		err  error
		want apperr.Kind
	}{
		{"nil", nil, apperr.KindUnknown},
		{"plain", errors.New("x"), apperr.KindUnknown},
		{"new", apperr.New(apperr.KindNotFound, "Не найдено"), apperr.KindNotFound},
		{"wrap", apperr.Wrap(apperr.KindConflict, "Занято", cause), apperr.KindConflict},
		{"validation", apperr.Validation("title", "Название: от 3 до 120 символов"), apperr.KindValidation},
		{"fmt wrapped", fmt.Errorf("op: %w", apperr.New(apperr.KindForbidden, "Нет доступа")), apperr.KindForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := apperr.KindOf(tt.err); got != tt.want {
				t.Fatalf("KindOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorUnwrapAndMessage(t *testing.T) {
	t.Parallel()

	cause := errors.New("db down")
	err := apperr.Wrap(apperr.KindPrecondition, "Публикация закрыта", cause)
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is must see the cause")
	}
	if err.Unwrap() != cause {
		t.Fatal("Unwrap must return the cause")
	}
	if got, want := err.Error(), "precondition: Публикация закрыта: db down"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	v := apperr.Validation("title", "слишком короткое")
	if v.Kind != apperr.KindValidation || v.Field != "title" || v.Msg != "слишком короткое" {
		t.Fatalf("unexpected validation error: %+v", v)
	}
	if got, want := v.Error(), "validation: title: слишком короткое"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	var target *apperr.Error
	if !errors.As(fmt.Errorf("wrap: %w", v), &target) || target != v {
		t.Fatal("errors.As must find *Error")
	}
}
