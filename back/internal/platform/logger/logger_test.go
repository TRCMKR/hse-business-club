package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
)

func TestFromContextEnrichment(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l, err := logger.NewWithWriter("debug", &buf)
	if err != nil {
		t.Fatal(err)
	}

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3},
		SpanID:     trace.SpanID{4, 5, 6},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	ctx = logger.WithContext(ctx, l)
	ctx = logger.WithRequestID(ctx, "req-1")
	ctx = logger.WithUserID(ctx, "user-1")

	logger.FromContext(ctx).Info("hello")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log is not JSON: %v: %s", err, buf.String())
	}
	want := map[string]string{
		"message":           "hello",
		"level":             "info",
		logger.KeyTraceID:   sc.TraceID().String(),
		logger.KeySpanID:    sc.SpanID().String(),
		logger.KeyRequestID: "req-1",
		logger.KeyUserID:    "user-1",
	}
	for k, v := range want {
		if entry[k] != v {
			t.Errorf("%s = %v, want %v", k, entry[k], v)
		}
	}
}

func TestFromContextFallback(t *testing.T) {
	t.Parallel()

	if logger.FromContext(context.Background()) == nil {
		t.Fatal("FromContext must never return nil")
	}
	if _, err := logger.ParseLevel("verbose"); err == nil {
		t.Fatal("ParseLevel must reject unknown levels")
	}
}
