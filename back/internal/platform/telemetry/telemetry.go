// Package telemetry wires OpenTelemetry tracing and the Prometheus registry and
// provides instrumentation helpers for gRPC, HTTP, pgx and pgxpool.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"

	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
)

// Build information, set via -ldflags "-X .../telemetry.Version=... -X .../telemetry.Commit=...".
var (
	Version = "dev"
	Commit  = "unknown"
)

// Telemetry owns the tracer provider and the metrics registry.
type Telemetry struct {
	tp       *sdktrace.TracerProvider
	registry *prometheus.Registry
	grpc     *grpcprom.ServerMetrics
	http     *httpMetrics
	exporter bool
}

// Init creates the tracer provider (OTLP/gRPC exporter when cfg.OTLPEndpoint is
// set, otherwise no exporter: spans are still created so trace_id reaches logs),
// installs it and the W3C propagator globally, and creates a Prometheus registry
// with Go runtime, process, build_info, gRPC server and HTTP metrics.
func Init(ctx context.Context, cfg config.Telemetry) (*Telemetry, error) {
	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "hbc-back"
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", Version),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry: resource: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerRatio))),
	}
	t := &Telemetry{registry: prometheus.NewRegistry()}
	if cfg.OTLPEndpoint != "" {
		exp, err := newExporter(ctx, cfg.OTLPEndpoint)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
		t.exporter = true
	}
	t.tp = sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(t.tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "build_info",
		Help: "Build information of the running binary.",
	}, []string{"version", "commit", "goversion"})
	buildInfo.WithLabelValues(Version, Commit, runtime.Version()).Set(1)

	t.grpc = grpcprom.NewServerMetrics(grpcprom.WithServerHandlingTimeHistogram())
	t.http = newHTTPMetrics()

	if err := t.Register(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		buildInfo,
		t.grpc,
		t.http.requests,
		t.http.duration,
	); err != nil {
		return nil, err
	}
	return t, nil
}

func newExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	var opt otlptracegrpc.Option
	if strings.Contains(endpoint, "://") {
		opt = otlptracegrpc.WithEndpointURL(endpoint)
	} else {
		opt = otlptracegrpc.WithEndpoint(endpoint)
	}
	exp, err := otlptracegrpc.New(ctx, opt, otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("telemetry: otlp exporter: %w", err)
	}
	return exp, nil
}

// Exporting reports whether spans are exported over OTLP.
func (t *Telemetry) Exporting() bool {
	return t.exporter
}

// TracerProvider returns the tracer provider.
func (t *Telemetry) TracerProvider() trace.TracerProvider {
	return t.tp
}

// Tracer returns a named tracer.
func (t *Telemetry) Tracer(name string) trace.Tracer {
	return t.tp.Tracer(name)
}

// Registry returns the Prometheus registry served on DEBUG_ADDR/metrics.
func (t *Telemetry) Registry() *prometheus.Registry {
	return t.registry
}

// Register registers collectors in the registry. Already registered collectors are ignored.
func (t *Telemetry) Register(cs ...prometheus.Collector) error {
	for _, c := range cs {
		if err := t.registry.Register(c); err != nil {
			var are prometheus.AlreadyRegisteredError
			if errors.As(err, &are) {
				continue
			}
			return fmt.Errorf("telemetry: register collector: %w", err)
		}
	}
	return nil
}

// Shutdown flushes pending spans and stops the tracer provider.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if err := t.tp.Shutdown(ctx); err != nil {
		return fmt.Errorf("telemetry: shutdown: %w", err)
	}
	return nil
}
