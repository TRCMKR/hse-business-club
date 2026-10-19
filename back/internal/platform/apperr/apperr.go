// Package apperr defines the application error type shared by all bounded
// contexts. Contexts return *Error; the BFF maps Kind to gRPC codes in one place.
package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies an error for transport mapping.
type Kind int

// Error kinds. Comments show the gRPC code / HTTP status the BFF maps them to.
const (
	KindUnknown         Kind = iota
	KindValidation           // InvalidArgument / 400
	KindUnauthenticated      // 401
	KindForbidden            // 403
	KindNotFound             // 404
	KindConflict             // AlreadyExists / 409
	KindPrecondition         // FailedPrecondition / 400
)

// String returns a short name of the kind, for logs.
func (k Kind) String() string {
	switch k {
	case KindValidation:
		return "validation"
	case KindUnauthenticated:
		return "unauthenticated"
	case KindForbidden:
		return "forbidden"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindPrecondition:
		return "precondition"
	default:
		return "unknown"
	}
}

// Error is an application error.
type Error struct {
	Kind  Kind
	Msg   string // ready to be shown to the user, in Russian
	Field string // for KindValidation, optional
	Err   error  // cause, for logs
}

// Error implements error. It includes the cause, so it is meant for logs, not users.
func (e *Error) Error() string {
	var s string
	if e.Field != "" {
		s = fmt.Sprintf("%s: %s: %s", e.Kind, e.Field, e.Msg)
	} else {
		s = fmt.Sprintf("%s: %s", e.Kind, e.Msg)
	}
	if e.Err != nil {
		s += ": " + e.Err.Error()
	}
	return s
}

// Unwrap returns the cause.
func (e *Error) Unwrap() error {
	return e.Err
}

// New creates an error of the given kind with a user-facing message.
func New(kind Kind, msg string) *Error {
	return &Error{Kind: kind, Msg: msg}
}

// Wrap creates an error of the given kind with a user-facing message and a cause.
func Wrap(kind Kind, msg string, err error) *Error {
	return &Error{Kind: kind, Msg: msg, Err: err}
}

// Validation creates a KindValidation error for the given field.
func Validation(field, msg string) *Error {
	return &Error{Kind: KindValidation, Msg: msg, Field: field}
}

// KindOf returns the kind of the first *Error in err's chain, KindUnknown if there is none.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindUnknown
}
