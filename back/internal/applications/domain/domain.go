// Package domain holds the applications context invariants: message validation and
// the application status state machine (SPEC §2.2–2.4).
package domain

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

// Message length limits, in characters (runes).
const (
	MessageMinLen = 1
	MessageMaxLen = 2000
)

// User-facing error messages (SPEC §2.3, §2.4).
const (
	MsgMessageLength  = "Сообщение: от 1 до 2000 символов"
	MsgAlreadyDecided = "Решение по отклику уже принято"
	MsgOwnListing     = "Нельзя откликнуться на свою публикацию"
	MsgListingClosed  = "Публикация закрыта"
	MsgAlreadyApplied = "Вы уже откликались на эту публикацию"
	MsgNotFound       = "Отклик не найден"
	MsgForbidden      = "Недостаточно прав"
	MsgStatusUnknown  = "Статус отклика: неизвестное значение"
	fieldMessage      = "message"
	fieldStatus       = "status"
)

// NormalizeMessage trims surrounding whitespace and checks the length (1–2000 characters).
// It returns the message to store.
func NormalizeMessage(message string) (string, error) {
	m := strings.TrimSpace(message)
	if n := utf8.RuneCountInString(m); n < MessageMinLen || n > MessageMaxLen {
		return "", apperr.Validation(fieldMessage, MsgMessageLength)
	}
	return m, nil
}

// ValidateStatusFilter accepts "" (all statuses) or one of the known statuses.
func ValidateStatusFilter(s applications.Status) error {
	if s == "" || IsKnown(s) {
		return nil
	}
	return apperr.Validation(fieldStatus, MsgStatusUnknown)
}

// IsKnown reports whether s is one of the four application statuses.
func IsKnown(s applications.Status) bool {
	switch s {
	case applications.StatusPending, applications.StatusAccepted,
		applications.StatusDeclined, applications.StatusWithdrawn:
		return true
	}
	return false
}

// Action is a status-changing operation on an application.
type Action int

// Actions.
const (
	ActionAccept Action = iota + 1
	ActionDecline
	ActionWithdraw
)

// String returns the action name, for logs.
func (a Action) String() string {
	switch a {
	case ActionAccept:
		return "accept"
	case ActionDecline:
		return "decline"
	case ActionWithdraw:
		return "withdraw"
	default:
		return fmt.Sprintf("action(%d)", int(a))
	}
}

// Target returns the status the action leads to from pending.
func (a Action) Target() (applications.Status, bool) {
	switch a {
	case ActionAccept:
		return applications.StatusAccepted, true
	case ActionDecline:
		return applications.StatusDeclined, true
	case ActionWithdraw:
		return applications.StatusWithdrawn, true
	default:
		return "", false
	}
}

// IsDecision reports whether the action is a decision of the listing author
// (accept/decline), which sets decided_at.
func (a Action) IsDecision() bool {
	return a == ActionAccept || a == ActionDecline
}

// Transition applies action to an application in status from (SPEC §2.2–2.3):
//   - pending → target of the action;
//   - repeating the same transition (already in the target status) → noop=true;
//   - from a final status to a different final status → KindPrecondition
//     «Решение по отклику уже принято».
//
// Unknown statuses or actions are programming errors (KindUnknown).
func Transition(from applications.Status, action Action) (to applications.Status, noop bool, err error) {
	target, ok := action.Target()
	if !ok {
		return "", false, apperr.New(apperr.KindUnknown, "unknown application action "+action.String())
	}
	switch {
	case from == applications.StatusPending:
		return target, false, nil
	case from == target:
		return from, true, nil
	case IsKnown(from):
		return "", false, apperr.New(apperr.KindPrecondition, MsgAlreadyDecided)
	default:
		return "", false, apperr.New(apperr.KindUnknown, fmt.Sprintf("unknown application status %q", from))
	}
}
