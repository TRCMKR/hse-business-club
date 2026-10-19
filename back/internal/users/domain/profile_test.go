package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
)

func assertValidation(t *testing.T, err error, field, msg string) {
	t.Helper()
	if msg == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	var e *apperr.Error
	if !errors.As(err, &e) {
		t.Fatalf("want validation %s %q, got %v", field, msg, err)
	}
	if e.Kind != apperr.KindValidation || e.Field != field || e.Msg != msg {
		t.Fatalf("want validation %s %q, got %+v", field, msg, e)
	}
}

func TestNormalizeRegister(t *testing.T) {
	valid := users.RegisterParams{Username: "anna_k", Password: "secret123", DisplayName: "Анна"}
	with := func(f func(p *users.RegisterParams)) users.RegisterParams {
		p := valid
		f(&p)
		return p
	}
	tests := []struct {
		name  string
		in    users.RegisterParams
		want  users.RegisterParams
		field string
		msg   string
	}{
		{name: "valid", in: valid, want: valid},
		{
			name: "username lower-cased and trimmed, display name trimmed",
			in:   with(func(p *users.RegisterParams) { p.Username = "  Anna_K "; p.DisplayName = " Анна " }),
			want: valid,
		},
		{name: "password kept as is", in: with(func(p *users.RegisterParams) { p.Password = " secret1 " }),
			want: with(func(p *users.RegisterParams) { p.Password = " secret1 " })},
		{name: "username 3", in: with(func(p *users.RegisterParams) { p.Username = "abc" }),
			want: with(func(p *users.RegisterParams) { p.Username = "abc" })},
		{name: "username 32", in: with(func(p *users.RegisterParams) { p.Username = strings.Repeat("a", 32) }),
			want: with(func(p *users.RegisterParams) { p.Username = strings.Repeat("a", 32) })},
		{name: "username short", in: with(func(p *users.RegisterParams) { p.Username = "ab" }), field: domain.FieldUsername, msg: domain.MsgUsername},
		{name: "username long", in: with(func(p *users.RegisterParams) { p.Username = strings.Repeat("a", 33) }), field: domain.FieldUsername, msg: domain.MsgUsername},
		{name: "username dash", in: with(func(p *users.RegisterParams) { p.Username = "anna-k" }), field: domain.FieldUsername, msg: domain.MsgUsername},
		{name: "username cyrillic", in: with(func(p *users.RegisterParams) { p.Username = "анна" }), field: domain.FieldUsername, msg: domain.MsgUsername},
		{name: "username inner space", in: with(func(p *users.RegisterParams) { p.Username = "an na" }), field: domain.FieldUsername, msg: domain.MsgUsername},
		{name: "password short", in: with(func(p *users.RegisterParams) { p.Password = "1234567" }), field: domain.FieldPassword, msg: domain.MsgPassword},
		{name: "password 72", in: with(func(p *users.RegisterParams) { p.Password = strings.Repeat("x", 72) }),
			want: with(func(p *users.RegisterParams) { p.Password = strings.Repeat("x", 72) })},
		{name: "password 73", in: with(func(p *users.RegisterParams) { p.Password = strings.Repeat("x", 73) }), field: domain.FieldPassword, msg: domain.MsgPassword},
		{name: "password over 72 bytes", in: with(func(p *users.RegisterParams) { p.Password = strings.Repeat("я", 40) }), field: domain.FieldPassword, msg: domain.MsgPassword},
		{name: "display name empty", in: with(func(p *users.RegisterParams) { p.DisplayName = "   " }), field: domain.FieldDisplayName, msg: domain.MsgDisplayName},
		{name: "display name 80 runes", in: with(func(p *users.RegisterParams) { p.DisplayName = strings.Repeat("я", 80) }),
			want: with(func(p *users.RegisterParams) { p.DisplayName = strings.Repeat("я", 80) })},
		{name: "display name 81", in: with(func(p *users.RegisterParams) { p.DisplayName = strings.Repeat("я", 81) }), field: domain.FieldDisplayName, msg: domain.MsgDisplayName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeRegister(tt.in)
			assertValidation(t, err, tt.field, tt.msg)
			if tt.msg == "" && got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeProfile(t *testing.T) {
	valid := users.UpdateProfileParams{
		DisplayName: "Анна", Headline: "Продакт", About: "О себе", Skills: []string{"Go", "SQL"}, Telegram: "anna",
	}
	with := func(f func(p *users.UpdateProfileParams)) users.UpdateProfileParams {
		p := valid
		p.Skills = append([]string(nil), valid.Skills...)
		f(&p)
		return p
	}
	many := func(n int) []string {
		s := make([]string, n)
		for i := range s {
			s[i] = "skill" + strings.Repeat("x", i)
		}
		return s
	}
	tests := []struct {
		name  string
		in    users.UpdateProfileParams
		want  users.UpdateProfileParams
		field string
		msg   string
	}{
		{name: "valid", in: valid, want: valid},
		{
			name: "trim everything, drop @ and empty skills",
			in: users.UpdateProfileParams{
				DisplayName: " Анна ", Headline: " Продакт ", About: "\nО себе\n",
				Skills: []string{" Go ", "", "  ", "SQL"}, Telegram: " @anna ",
			},
			want: valid,
		},
		{name: "only first @ dropped", in: with(func(p *users.UpdateProfileParams) { p.Telegram = "@@anna" }),
			want: with(func(p *users.UpdateProfileParams) { p.Telegram = "@anna" })},
		{name: "empty optional fields, nil skills", in: users.UpdateProfileParams{DisplayName: "A"},
			want: users.UpdateProfileParams{DisplayName: "A", Skills: []string{}}},
		{name: "display name empty", in: with(func(p *users.UpdateProfileParams) { p.DisplayName = "" }), field: domain.FieldDisplayName, msg: domain.MsgDisplayName},
		{name: "headline 120", in: with(func(p *users.UpdateProfileParams) { p.Headline = strings.Repeat("ю", 120) }),
			want: with(func(p *users.UpdateProfileParams) { p.Headline = strings.Repeat("ю", 120) })},
		{name: "headline 121", in: with(func(p *users.UpdateProfileParams) { p.Headline = strings.Repeat("ю", 121) }), field: domain.FieldHeadline, msg: domain.MsgHeadline},
		{name: "about 2001", in: with(func(p *users.UpdateProfileParams) { p.About = strings.Repeat("a", 2001) }), field: domain.FieldAbout, msg: domain.MsgAbout},
		{name: "skills 20", in: with(func(p *users.UpdateProfileParams) { p.Skills = many(20) }),
			want: with(func(p *users.UpdateProfileParams) { p.Skills = many(20) })},
		{name: "skills 21", in: with(func(p *users.UpdateProfileParams) { p.Skills = many(21) }), field: domain.FieldSkills, msg: domain.MsgSkillsCount},
		{name: "skills 20 plus empties", in: with(func(p *users.UpdateProfileParams) { p.Skills = append(many(20), "", " ") }),
			want: with(func(p *users.UpdateProfileParams) { p.Skills = many(20) })},
		{name: "skill 41", in: with(func(p *users.UpdateProfileParams) { p.Skills = []string{strings.Repeat("я", 41)} }), field: domain.FieldSkills, msg: domain.MsgSkillLength},
		{name: "skill 40 after trim", in: with(func(p *users.UpdateProfileParams) { p.Skills = []string{"  " + strings.Repeat("я", 40) + "  "} }),
			want: with(func(p *users.UpdateProfileParams) { p.Skills = []string{strings.Repeat("я", 40)} })},
		{name: "duplicate skills case-insensitive", in: with(func(p *users.UpdateProfileParams) { p.Skills = []string{"Go", " go "} }), field: domain.FieldSkills, msg: domain.MsgSkillsDup},
		{name: "duplicate cyrillic skills", in: with(func(p *users.UpdateProfileParams) { p.Skills = []string{"Маркетинг", "маркетинг"} }), field: domain.FieldSkills, msg: domain.MsgSkillsDup},
		{name: "telegram 64 after @", in: with(func(p *users.UpdateProfileParams) { p.Telegram = "@" + strings.Repeat("a", 64) }),
			want: with(func(p *users.UpdateProfileParams) { p.Telegram = strings.Repeat("a", 64) })},
		{name: "telegram 65", in: with(func(p *users.UpdateProfileParams) { p.Telegram = strings.Repeat("a", 65) }), field: domain.FieldTelegram, msg: domain.MsgTelegram},
		{name: "invalid utf-8", in: with(func(p *users.UpdateProfileParams) { p.About = "\xff" }), field: domain.FieldAbout, msg: domain.MsgAbout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NormalizeProfile(tt.in)
			assertValidation(t, err, tt.field, tt.msg)
			if tt.msg == "" && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeUsername(t *testing.T) {
	if got := domain.NormalizeUsername("  Anna_K\t"); got != "anna_k" {
		t.Fatalf("got %q", got)
	}
}
