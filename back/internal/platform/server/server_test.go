package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/trcmkr/hse-business-club/back/internal/bff"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
	"github.com/trcmkr/hse-business-club/back/internal/platform/server"
	"github.com/trcmkr/hse-business-club/back/internal/platform/telemetry"
)

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type env struct {
	srv    *server.Server
	tel    *telemetry.Telemetry
	logs   *syncBuffer
	public *httptest.Server
	debug  *httptest.Server
	jwt    *auth.JWT
}

func start(t *testing.T, blobErr error) *env {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	logs := &syncBuffer{}
	log, err := logger.NewWithWriter("debug", logs)
	if err != nil {
		t.Fatal(err)
	}
	tel, err := telemetry.Init(ctx, config.Telemetry{ServiceName: "test", SamplerRatio: 1})
	if err != nil {
		t.Fatal(err)
	}
	jwt := auth.NewJWT("test-secret", time.Hour)

	const anyPort = "127.0.0.1:0"
	cfg := config.Server{
		HTTPAddr:        anyPort,
		DebugAddr:       anyPort,
		GRPCAddr:        anyPort,
		CORSOrigins:     []string{"http://localhost:3000"},
		ShutdownTimeout: 5 * time.Second,
	}
	srv, err := server.New(ctx, cfg, log, tel, jwt, bff.New(bff.Deps{Issuer: jwt}))
	if err != nil {
		t.Fatal(err)
	}
	srv.AddReadinessCheck("db", func(context.Context) error { return nil })
	srv.AddReadinessCheck("blob", func(context.Context) error { return blobErr })

	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	e := &env{
		srv:    srv,
		tel:    tel,
		logs:   logs,
		public: httptest.NewServer(srv.PublicHandler()),
		debug:  httptest.NewServer(srv.DebugHandler()),
		jwt:    jwt,
	}
	t.Cleanup(func() {
		e.public.Close()
		e.debug.Close()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run() error = %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("Run() did not stop")
		}
		_ = tel.Shutdown(context.Background())
	})
	return e
}

func get(t *testing.T, url string, header map[string]string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(body)
}

func TestPingThroughGateway(t *testing.T) {
	e := start(t, nil)

	code, hdr, body := get(t, e.public.URL+"/v1/ping", nil)
	if code != http.StatusOK || !jsonEqual(t, body, `{"ok":true}`) {
		t.Fatalf("GET /v1/ping = %d %s", code, body)
	}
	if hdr.Get("X-Request-Id") == "" {
		t.Error("X-Request-Id response header is missing")
	}

	// The call went through the gRPC hop: the prometheus interceptor counted it...
	const series = `grpc_server_handled_total{grpc_code="OK",grpc_method="V1Ping",grpc_service="hbc.v1.HseBusinessClub",grpc_type="unary"} 1`
	if m := scrape(t, e); !strings.Contains(m, series) {
		t.Fatalf("V1Ping not counted:\n%s", grep(m, "V1Ping"))
	}

	// ...and the logging interceptor logged it with the request id.
	logs := e.logs.String()
	if !strings.Contains(logs, `"grpc.method":"/hbc.v1.HseBusinessClub/V1Ping"`) || !strings.Contains(logs, `"request_id":"`) {
		t.Fatalf("logging interceptor did not log V1Ping with request_id:\n%s", logs)
	}

	// HTTP metrics labelled by route template.
	if m := scrape(t, e); !strings.Contains(m, `http_requests_total{method="GET",route="/v1/ping",status="200"} 1`) {
		t.Fatalf("http_requests_total for /v1/ping missing:\n%s", grep(m, "http_requests_total"))
	}
}

func TestAuthRequired(t *testing.T) {
	e := start(t, nil)

	code, _, body := get(t, e.public.URL+"/v1/me", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("GET /v1/me without token = %d %s", code, body)
	}
	if want := `{"code":16,"message":"Требуется вход","details":[]}`; !jsonEqual(t, body, want) {
		t.Fatalf("body = %s, want %s", body, want)
	}

	// A valid token passes the interceptor and reaches the (unimplemented) BFF method.
	tok, err := e.jwt.Issue([16]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	code, _, body = get(t, e.public.URL+"/v1/me", map[string]string{"Authorization": "Bearer " + tok})
	if code != http.StatusNotImplemented {
		t.Fatalf("GET /v1/me with token = %d %s, want 501", code, body)
	}
}

func TestPublicAvatarReachesBFF(t *testing.T) {
	e := start(t, nil)

	code, _, body := get(t, e.public.URL+"/v1/avatars/x.jpg", nil)
	if code != http.StatusNotImplemented {
		t.Fatalf("GET /v1/avatars/x.jpg = %d %s, want 501 (public method)", code, body)
	}
}

func TestUnknownRoute(t *testing.T) {
	e := start(t, nil)

	code, _, body := get(t, e.public.URL+"/v1/nope", nil)
	if code != http.StatusNotFound || !jsonEqual(t, body, `{"code":5,"message":"Не найдено","details":[]}`) {
		t.Fatalf("GET /v1/nope = %d %s", code, body)
	}
}

func TestHealthAndReadiness(t *testing.T) {
	e := start(t, nil)

	for _, base := range []string{e.public.URL, e.debug.URL} {
		if code, _, body := get(t, base+"/healthz", nil); code != http.StatusOK {
			t.Fatalf("healthz = %d %s", code, body)
		}
		code, _, body := get(t, base+"/readyz", nil)
		if code != http.StatusOK || body != `{"db":"ok","blob":"ok"}` {
			t.Fatalf("readyz = %d %s", code, body)
		}
	}
}

func TestReadinessFailure(t *testing.T) {
	e := start(t, errors.New("bucket missing"))

	code, _, body := get(t, e.public.URL+"/readyz", nil)
	if code != http.StatusServiceUnavailable || body != `{"db":"ok","blob":"bucket missing"}` {
		t.Fatalf("readyz = %d %s", code, body)
	}
}

func TestDebugEndpoints(t *testing.T) {
	e := start(t, nil)

	checks := []struct {
		path, contains string
	}{
		{"/metrics", "build_info"},
		{"/docs/", "swagger-ui"},
		{"/docs/swagger.json", `"/v1/ping"`},
		{"/debug/pprof/", "goroutine"},
		{"/v1/ping", `true`},
	}
	for _, c := range checks {
		code, _, body := get(t, e.debug.URL+c.path, nil)
		if code != http.StatusOK || !strings.Contains(body, c.contains) {
			t.Errorf("GET debug %s = %d, body must contain %q", c.path, code, c.contains)
		}
	}
}

func TestCORS(t *testing.T) {
	e := start(t, nil)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodOptions, e.public.URL+"/v1/me", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "authorization")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func scrape(t *testing.T, e *env) string {
	t.Helper()
	_, _, body := get(t, e.debug.URL+"/metrics", nil)
	return body
}

func grep(s, substr string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, substr) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// jsonEqual compares JSON semantically (protojson output has unstable whitespace).
func jsonEqual(t *testing.T, got, want string) bool {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Errorf("invalid JSON %q: %v", got, err)
		return false
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("invalid expected JSON %q: %v", want, err)
	}
	return reflect.DeepEqual(g, w)
}
