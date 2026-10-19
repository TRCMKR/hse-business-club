package postgres_test

import (
	"testing"

	"github.com/trcmkr/hse-business-club/back/internal/listings/postgres"
)

func TestEscapeLike(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"":           "",
		"go":         "go",
		"10%":        `10\%`,
		"snake_case": `snake\_case`,
		`C:\dev`:     `C:\\dev`,
		`\%_`:        `\\\%\_`,
	}
	for in, want := range tests {
		if got := postgres.EscapeLike(in); got != want {
			t.Errorf("EscapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}
