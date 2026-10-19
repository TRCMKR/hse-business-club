// Package logger builds the zap JSON logger and carries it through context.
// FromContext enriches the logger with trace_id, span_id, request_id and user_id.
package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Field keys added by FromContext.
const (
	KeyTraceID   = "trace_id"
	KeySpanID    = "span_id"
	KeyRequestID = "request_id"
	KeyUserID    = "user_id"
)

type ctxKey int

const (
	loggerKey ctxKey = iota
	requestIDKey
	userIDKey
)

var global atomic.Pointer[zap.Logger]

func init() {
	global.Store(zap.NewNop())
}

// New creates a JSON logger writing to stdout. level is one of debug, info, warn, error.
func New(level string) (*zap.Logger, error) {
	return NewWithWriter(level, os.Stdout)
}

// NewWithWriter creates a JSON logger writing to w (useful in tests).
func NewWithWriter(level string, w io.Writer) (*zap.Logger, error) {
	lvl, err := ParseLevel(level)
	if err != nil {
		return nil, err
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}),
		zapcore.AddSync(w),
		zap.NewAtomicLevelAt(lvl),
	)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel)), nil
}

// ParseLevel parses a LOG_LEVEL value.
func ParseLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zap.DebugLevel, nil
	case "", "info":
		return zap.InfoLevel, nil
	case "warn":
		return zap.WarnLevel, nil
	case "error":
		return zap.ErrorLevel, nil
	default:
		return zap.InfoLevel, fmt.Errorf("logger: unknown level %q", level)
	}
}

// SetGlobal sets the logger returned by L and used by FromContext as a fallback.
func SetGlobal(l *zap.Logger) {
	if l != nil {
		global.Store(l)
	}
}

// L returns the global logger (no-op until SetGlobal is called).
func L() *zap.Logger {
	return global.Load()
}

// WithContext returns a copy of ctx carrying l.
func WithContext(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// WithRequestID returns a copy of ctx carrying the request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request id stored by WithRequestID.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// WithUserID returns a copy of ctx carrying the authenticated user id.
func WithUserID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, userIDKey, id)
}

// FromContext returns the logger stored in ctx (or the global one) enriched with
// trace_id/span_id of the current span, request_id and user_id when present.
func FromContext(ctx context.Context) *zap.Logger {
	l, ok := ctx.Value(loggerKey).(*zap.Logger)
	if !ok || l == nil {
		l = L()
	}
	fields := make([]zap.Field, 0, 4)
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		fields = append(fields,
			zap.String(KeyTraceID, sc.TraceID().String()),
			zap.String(KeySpanID, sc.SpanID().String()),
		)
	}
	if id := RequestIDFromContext(ctx); id != "" {
		fields = append(fields, zap.String(KeyRequestID, id))
	}
	if id, ok := ctx.Value(userIDKey).(string); ok && id != "" {
		fields = append(fields, zap.String(KeyUserID, id))
	}
	if len(fields) == 0 {
		return l
	}
	return l.With(fields...)
}
