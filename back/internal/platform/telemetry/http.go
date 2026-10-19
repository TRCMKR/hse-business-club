package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

const unmatchedRoute = "unmatched"

type httpMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func newHTTPMetrics() *httpMetrics {
	return &httpMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by route, method and status.",
		}, []string{"route", "method", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration by route, method and status.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method", "status"}),
	}
}

// HTTPMiddleware returns chi middleware that starts an otelhttp server span and
// records http_requests_total / http_request_duration_seconds. The route label and
// the span name use the chi route pattern, so it must be used on a chi router.
// /healthz and /readyz are not traced.
func (t *Telemetry) HTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		measured := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			route := unmatchedRoute
			if rc := chi.RouteContext(r.Context()); rc != nil {
				if p := rc.RoutePattern(); p != "" {
					route = p
				}
			}
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			trace.SpanFromContext(r.Context()).SetName(r.Method + " " + route)

			code := strconv.Itoa(status)
			t.http.requests.WithLabelValues(route, r.Method, code).Inc()
			t.http.duration.WithLabelValues(route, r.Method, code).Observe(time.Since(start).Seconds())
		})
		return otelhttp.NewHandler(measured, "http.server",
			otelhttp.WithTracerProvider(t.tp),
			otelhttp.WithFilter(func(r *http.Request) bool {
				return r.URL.Path != "/healthz" && r.URL.Path != "/readyz"
			}),
		)
	}
}

// HTTPTransport wraps base (http.DefaultTransport when nil) with otelhttp client tracing.
func (t *Telemetry) HTTPTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base, otelhttp.WithTracerProvider(t.tp))
}
