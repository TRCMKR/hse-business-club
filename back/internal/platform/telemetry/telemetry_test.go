package telemetry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/telemetry"
)

func TestInitAndHTTPMiddleware(t *testing.T) {
	ctx := context.Background()
	tel, err := telemetry.Init(ctx, config.Telemetry{ServiceName: "test", SamplerRatio: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tel.Shutdown(ctx) })

	if tel.Exporting() {
		t.Fatal("no endpoint must mean no exporter")
	}

	r := chi.NewRouter()
	r.Use(tel.HTTPMiddleware())
	r.Get("/v1/users/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/users/42", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d", rec.Code)
	}

	n, err := testutil.GatherAndCount(tel.Registry(), "http_requests_total", "build_info")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected one http_requests_total series and build_info, got %d", n)
	}
	mfs, err := tel.Registry().Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "http_requests_total" {
			continue
		}
		for _, lp := range mf.GetMetric()[0].GetLabel() {
			if lp.GetName() == "route" && lp.GetValue() != "/v1/users/{id}" {
				t.Fatalf("route label = %q", lp.GetValue())
			}
		}
	}
}
