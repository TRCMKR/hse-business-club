package bff

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
)

// msgInternal hides details of unexpected errors from the user (SPEC §4.2).
const msgInternal = "Внутренняя ошибка"

// kindCodes maps apperr kinds to gRPC codes (SPEC §4.2).
var kindCodes = map[apperr.Kind]codes.Code{
	apperr.KindValidation:      codes.InvalidArgument,
	apperr.KindUnauthenticated: codes.Unauthenticated,
	apperr.KindForbidden:       codes.PermissionDenied,
	apperr.KindNotFound:        codes.NotFound,
	apperr.KindConflict:        codes.AlreadyExists,
	apperr.KindPrecondition:    codes.FailedPrecondition,
}

// toStatus converts an error returned by a context into a gRPC status error.
// *apperr.Error with a known kind keeps its Msg; anything else becomes Internal
// «Внутренняя ошибка» and the original error is logged with trace_id.
// Errors that already are gRPC statuses pass through unchanged.
func toStatus(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var ae *apperr.Error
	if errors.As(err, &ae) {
		if code, ok := kindCodes[ae.Kind]; ok {
			return status.Error(code, ae.Msg)
		}
	} else if _, ok := status.FromError(err); ok {
		return err
	}
	logger.FromContext(ctx).Error("internal error", zap.Error(err))
	return status.Error(codes.Internal, msgInternal)
}
