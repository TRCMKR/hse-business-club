package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/applications/domain"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

func TestTransition(t *testing.T) {
	const (
		pending   = applications.StatusPending
		accepted  = applications.StatusAccepted
		declined  = applications.StatusDeclined
		withdrawn = applications.StatusWithdrawn
	)
	type want struct {
		to   applications.Status
		noop bool
		kind apperr.Kind // KindUnknown with ok=true means no error
		ok   bool
	}
	okTo := func(s applications.Status) want { return want{to: s, ok: true} }
	noop := func(s applications.Status) want { return want{to: s, noop: true, ok: true} }
	decided := want{kind: apperr.KindPrecondition}

	cases := []struct {
		from   applications.Status
		action domain.Action
		want   want
	}{
		{pending, domain.ActionAccept, okTo(accepted)},
		{pending, domain.ActionDecline, okTo(declined)},
		{pending, domain.ActionWithdraw, okTo(withdrawn)},

		{accepted, domain.ActionAccept, noop(accepted)},
		{accepted, domain.ActionDecline, decided},
		{accepted, domain.ActionWithdraw, decided},

		{declined, domain.ActionAccept, decided},
		{declined, domain.ActionDecline, noop(declined)},
		{declined, domain.ActionWithdraw, decided},

		{withdrawn, domain.ActionAccept, decided},
		{withdrawn, domain.ActionDecline, decided},
		{withdrawn, domain.ActionWithdraw, noop(withdrawn)},
	}
	for _, tc := range cases {
		t.Run(string(tc.from)+"_"+tc.action.String(), func(t *testing.T) {
			to, isNoop, err := domain.Transition(tc.from, tc.action)
			if tc.want.ok {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if to != tc.want.to || isNoop != tc.want.noop {
					t.Fatalf("got (%q, noop=%v), want (%q, noop=%v)", to, isNoop, tc.want.to, tc.want.noop)
				}
				return
			}
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Kind != tc.want.kind {
				t.Fatalf("err = %v, want kind %v", err, tc.want.kind)
			}
			if ae.Msg != "Решение по отклику уже принято" {
				t.Fatalf("msg = %q", ae.Msg)
			}
		})
	}
}

func TestTransitionUnknown(t *testing.T) {
	if _, _, err := domain.Transition("bogus", domain.ActionAccept); apperr.KindOf(err) != apperr.KindUnknown || err == nil {
		t.Fatalf("unknown status: err = %v", err)
	}
	if _, _, err := domain.Transition(applications.StatusPending, domain.Action(42)); apperr.KindOf(err) != apperr.KindUnknown || err == nil {
		t.Fatalf("unknown action: err = %v", err)
	}
}

func TestNormalizeMessage(t *testing.T) {
	cases := []struct {
		name, in, want string
		ok             bool
	}{
		{"one char", "a", "a", true},
		{"trimmed", "  привет \n", "привет", true},
		{"max runes cyrillic", strings.Repeat("я", 2000), strings.Repeat("я", 2000), true},
		{"empty", "", "", false},
		{"only spaces", " \t\n ", "", false},
		{"too long", strings.Repeat("я", 2001), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.NormalizeMessage(tc.in)
			if !tc.ok {
				var ae *apperr.Error
				if !errors.As(err, &ae) || ae.Kind != apperr.KindValidation || ae.Field != "message" ||
					ae.Msg != "Сообщение: от 1 до 2000 символов" {
					t.Fatalf("err = %#v", err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got (%q, %v), want %q", got, err, tc.want)
			}
		})
	}
}

func TestValidateStatusFilter(t *testing.T) {
	for _, s := range []applications.Status{"", applications.StatusPending, applications.StatusAccepted,
		applications.StatusDeclined, applications.StatusWithdrawn} {
		if err := domain.ValidateStatusFilter(s); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
	}
	if err := domain.ValidateStatusFilter("PENDING"); apperr.KindOf(err) != apperr.KindValidation {
		t.Fatalf("err = %v", err)
	}
}
