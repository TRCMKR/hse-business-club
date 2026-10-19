// Package domain holds the users context rules: normalization and validation of
// profile fields (SPEC §2.4) and profile photo normalization. Pure Go, no I/O.
package domain

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
)

// Field limits from SPEC §2.4.
const (
	UsernameMin    = 3
	UsernameMax    = 32
	PasswordMin    = 8
	PasswordMax    = 72 // also the bcrypt input limit in bytes
	DisplayNameMax = 80
	HeadlineMax    = 120
	AboutMax       = 2000
	SkillsMax      = 20
	SkillMax       = 40
	TelegramMax    = 64
)

// Field names used in apperr.Validation (proto/JSON field names).
const (
	FieldUsername    = "username"
	FieldPassword    = "password"
	FieldDisplayName = "display_name"
	FieldHeadline    = "headline"
	FieldAbout       = "about"
	FieldSkills      = "skills"
	FieldTelegram    = "telegram"
	FieldAvatar      = "avatar"
)

// User-facing validation messages (labels from the SPEC §2.4 table).
const (
	MsgUsername    = "Логин: от 3 до 32 символов, латиница, цифры и _"
	MsgPassword    = "Пароль: от 8 до 72 символов"
	MsgDisplayName = "Имя: от 1 до 80 символов"
	MsgHeadline    = "Кратко о себе: не больше 120 символов"
	MsgAbout       = "О себе: не больше 2000 символов"
	MsgSkillsCount = "Навыки: не больше 20"
	MsgSkillLength = "Навыки: каждый от 1 до 40 символов"
	MsgSkillsDup   = "Навыки: без повторов"
	MsgTelegram    = "Telegram: не больше 64 символов"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9_]+$`)

// NormalizeUsername trims and lower-cases a login. It does not validate.
func NormalizeUsername(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ValidateUsername checks an already normalized login.
func ValidateUsername(s string) error {
	if n := len(s); n < UsernameMin || n > UsernameMax || !usernameRe.MatchString(s) {
		return apperr.Validation(FieldUsername, MsgUsername)
	}
	return nil
}

// ValidatePassword checks the password length. The password is used as is (no trim).
// Besides 8–72 characters, it must fit into 72 bytes: bcrypt rejects longer input.
func ValidatePassword(s string) error {
	n := utf8.RuneCountInString(s)
	if n < PasswordMin || n > PasswordMax || len(s) > PasswordMax || !utf8.ValidString(s) {
		return apperr.Validation(FieldPassword, MsgPassword)
	}
	return nil
}

// NormalizeRegister normalizes and validates registration input:
// username is trimmed and lower-cased, display name is trimmed, password is kept as is.
func NormalizeRegister(p users.RegisterParams) (users.RegisterParams, error) {
	out := users.RegisterParams{
		Username:    NormalizeUsername(p.Username),
		Password:    p.Password,
		DisplayName: strings.TrimSpace(p.DisplayName),
	}
	if err := ValidateUsername(out.Username); err != nil {
		return users.RegisterParams{}, err
	}
	if err := ValidatePassword(out.Password); err != nil {
		return users.RegisterParams{}, err
	}
	if err := validateDisplayName(out.DisplayName); err != nil {
		return users.RegisterParams{}, err
	}
	return out, nil
}

// NormalizeProfile normalizes and validates a profile update: strings are trimmed,
// the leading "@" of telegram is dropped, skills are trimmed with empty items removed;
// case-insensitive duplicates are an error. Skills of the result are never nil.
func NormalizeProfile(p users.UpdateProfileParams) (users.UpdateProfileParams, error) {
	out := users.UpdateProfileParams{
		DisplayName: strings.TrimSpace(p.DisplayName),
		Headline:    strings.TrimSpace(p.Headline),
		About:       strings.TrimSpace(p.About),
		Telegram:    strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p.Telegram), "@")),
	}
	if err := validateDisplayName(out.DisplayName); err != nil {
		return users.UpdateProfileParams{}, err
	}
	if !maxLen(out.Headline, HeadlineMax) {
		return users.UpdateProfileParams{}, apperr.Validation(FieldHeadline, MsgHeadline)
	}
	if !maxLen(out.About, AboutMax) {
		return users.UpdateProfileParams{}, apperr.Validation(FieldAbout, MsgAbout)
	}
	skills, err := NormalizeSkills(p.Skills)
	if err != nil {
		return users.UpdateProfileParams{}, err
	}
	out.Skills = skills
	if !maxLen(out.Telegram, TelegramMax) {
		return users.UpdateProfileParams{}, apperr.Validation(FieldTelegram, MsgTelegram)
	}
	return out, nil
}

// NormalizeSkills trims items, drops empty ones and checks count, length and
// case-insensitive uniqueness. The result is never nil.
func NormalizeSkills(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !maxLen(s, SkillMax) {
			return nil, apperr.Validation(FieldSkills, MsgSkillLength)
		}
		k := strings.ToLower(s)
		if _, ok := seen[k]; ok {
			return nil, apperr.Validation(FieldSkills, MsgSkillsDup)
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}
	if len(out) > SkillsMax {
		return nil, apperr.Validation(FieldSkills, MsgSkillsCount)
	}
	return out, nil
}

func validateDisplayName(s string) error {
	if s == "" || !maxLen(s, DisplayNameMax) {
		return apperr.Validation(FieldDisplayName, MsgDisplayName)
	}
	return nil
}

// maxLen reports whether s is valid UTF-8 of at most n characters.
func maxLen(s string, n int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= n
}
