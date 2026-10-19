//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/trcmkr/hse-business-club/back/internal/app"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob/memory"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
	"github.com/trcmkr/hse-business-club/back/internal/platform/telemetry"
)

// J is a decoded JSON object.
type J = map[string]any

// password is used for every user registered by the tests (same as the seed).
const password = "demo1234"

// env is one running application.
type env struct {
	t    *testing.T
	base string // http://127.0.0.1:<port>
	blob *memory.Store
	logs *syncBuffer
}

type options struct {
	seed bool
}

type option func(*options)

// withSeed starts the app with SEED_DEMO_DATA=true.
func withSeed() option { return func(o *options) { o.seed = true } }

// newEnv truncates all tables and starts the app on free ports. The app is
// stopped when the test ends; its logs are printed if the test failed.
func newEnv(t *testing.T, opts ...option) *env {
	t.Helper()
	truncate(t)
	return startApp(t, opts...)
}

// startApp starts one more app on the current database (no truncate).
func startApp(t *testing.T, opts ...option) *env {
	t.Helper()
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	ctx, cancel := context.WithCancel(context.Background())
	logs := &syncBuffer{}
	log, err := logger.NewWithWriter("debug", logs)
	if err != nil {
		t.Fatal(err)
	}
	// A fresh Telemetry per app: its own Prometheus registry, so collectors of
	// several apps in one process never clash (AlreadyRegistered is ignored).
	tel, err := telemetry.Init(ctx, config.Telemetry{ServiceName: "hbc-e2e", SamplerRatio: 1})
	if err != nil {
		t.Fatal(err)
	}
	const anyPort = "127.0.0.1:0"
	cfg := &config.Config{
		Env:      config.EnvTest,
		LogLevel: "debug",
		Server: config.Server{
			HTTPAddr: anyPort, DebugAddr: anyPort, GRPCAddr: anyPort,
			CORSOrigins:     []string{"http://localhost:3000"},
			ShutdownTimeout: 5 * time.Second,
		},
		Auth: config.Auth{JWTSecret: "e2e-secret-e2e-secret-e2e-secret", JWTTTL: time.Hour},
		Seed: config.Seed{DemoData: o.seed},
	}
	store := memory.New()
	a, err := app.New(ctx, cfg, app.Deps{Log: log, Telemetry: tel, Pool: pool, Blob: store})
	if err != nil {
		cancel()
		t.Fatalf("app.New: %v\n%s", err, logs.String())
	}
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("app.Run: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("app did not stop")
		}
		_ = tel.Shutdown(context.Background())
		if t.Failed() {
			t.Logf("app logs:\n%s", logs.String())
		}
	})
	return &env{t: t, base: "http://" + a.Server().HTTPAddr().String(), blob: store, logs: logs}
}

func truncate(t *testing.T) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), truncateSQL); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// raw sends a request and returns the response with its whole body.
// body is nil, a string (sent as is) or a value encoded as JSON.
func (e *env) raw(method, path, token string, body any) (*http.Response, []byte) {
	e.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	default:
		buf, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		r = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, e.base+path, r)
	if err != nil {
		e.t.Fatal(err)
	}
	if r != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return resp, data
}

// do sends a JSON request and decodes the JSON object of the response.
func (e *env) do(method, path, token string, body any) (int, J) {
	e.t.Helper()
	resp, data := e.raw(method, path, token, body)
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		e.t.Fatalf("%s %s: Content-Type %q, body %s", method, path, ct, data)
	}
	var out J
	if err := json.Unmarshal(data, &out); err != nil {
		e.t.Fatalf("%s %s: decode %s: %v", method, path, data, err)
	}
	return resp.StatusCode, out
}

// ok sends a request and requires 200.
func (e *env) ok(method, path, token string, body any) J {
	e.t.Helper()
	code, out := e.do(method, path, token, body)
	if code != http.StatusOK {
		e.t.Fatalf("%s %s = %d %v, want 200", method, path, code, out)
	}
	return out
}

// fails sends a request and requires an error in grpc-gateway format.
func (e *env) fails(method, path, token string, body any, httpStatus, grpcCode int, message string) {
	e.t.Helper()
	code, out := e.do(method, path, token, body)
	if code != httpStatus {
		e.t.Fatalf("%s %s = %d %v, want %d", method, path, code, out, httpStatus)
	}
	expectError(e.t, out, grpcCode, message)
}

// expectError checks {code, message, details: []} exactly.
func expectError(t *testing.T, out J, grpcCode int, message string) {
	t.Helper()
	expectKeys(t, "error", out, "code", "message", "details")
	if got := num(out["code"]); got != grpcCode {
		t.Errorf("error code = %d, want %d (%v)", got, grpcCode, out)
	}
	if out["message"] != message {
		t.Errorf("error message = %q, want %q", out["message"], message)
	}
	if d, ok := out["details"].([]any); !ok || len(d) != 0 {
		t.Errorf("error details = %v, want []", out["details"])
	}
}

// register creates a user (password demo1234) and returns the token and the user.
func (e *env) register(username, displayName string) (string, J) {
	e.t.Helper()
	out := e.ok(http.MethodPost, "/v1/auth/register", "", J{
		"username": username, "password": password, "displayName": displayName,
	})
	expectKeys(e.t, "auth response", out, "token", "user")
	tok, _ := out["token"].(string)
	if tok == "" {
		e.t.Fatalf("register %s: empty token", username)
	}
	return tok, obj(e.t, out, "user")
}

// login returns a token of username with password demo1234.
func (e *env) login(username string) string {
	e.t.Helper()
	out := e.ok(http.MethodPost, "/v1/auth/login", "", J{"username": username, "password": password})
	tok, _ := out["token"].(string)
	if tok == "" {
		e.t.Fatalf("login %s: empty token", username)
	}
	return tok
}

// startup and vacancy are valid create requests.
func startup(title string) J {
	return J{
		"kind": "LISTING_KIND_STARTUP", "title": title,
		"description": "Описание стартапа для e2e-теста", "lookingFor": "Сооснователь",
		"tags": []string{"Go", "Backend"}, "stage": "STARTUP_STAGE_MVP",
	}
}

func vacancy(title string) J {
	return J{
		"kind": "LISTING_KIND_VACANCY", "title": title,
		"description": "Описание вакансии для e2e-теста", "lookingFor": "Разработчик",
		"tags": []string{"Vue"}, "compensation": "Доля",
	}
}

// createListing creates a listing and returns it.
func (e *env) createListing(token string, req J) J {
	e.t.Helper()
	return e.ok(http.MethodPost, "/v1/listings", token, req)
}

// apply creates an application and returns it.
func (e *env) apply(token, listingID, message string) J {
	e.t.Helper()
	return e.ok(http.MethodPost, "/v1/listings/"+listingID+"/applications", token, J{"message": message})
}

// ---- JSON helpers ----

func str(t *testing.T, m J, key string) string {
	t.Helper()
	s, ok := m[key].(string)
	if !ok {
		t.Fatalf("%q is %T (%v), want string", key, m[key], m[key])
	}
	return s
}

func obj(t *testing.T, m J, key string) J {
	t.Helper()
	o, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%q is %T (%v), want object", key, m[key], m[key])
	}
	return o
}

func items(t *testing.T, m J) []J {
	t.Helper()
	arr, ok := m["items"].([]any)
	if !ok {
		t.Fatalf("items is %T (%v), want array", m["items"], m["items"])
	}
	out := make([]J, len(arr))
	for i, v := range arr {
		o, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("items[%d] is %T, want object", i, v)
		}
		out[i] = o
	}
	return out
}

func num(v any) int {
	f, ok := v.(float64)
	if !ok {
		return -1 << 31
	}
	return int(f)
}

func strs(v any) []string {
	arr, _ := v.([]any)
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

// expectKeys checks the exact key set of a JSON object (EmitUnpopulated: every
// field is present, camelCase).
func expectKeys(t *testing.T, what string, m J, keys ...string) {
	t.Helper()
	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	want := append([]string(nil), keys...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s keys = %v, want %v", what, got, want)
	}
}

var (
	userKeys    = []string{"id", "username", "displayName", "headline", "about", "skills", "telegram", "createdAt", "avatarUrl"}
	listingKeys = []string{"id", "kind", "title", "description", "lookingFor", "tags", "stage", "compensation", "status",
		"author", "createdAt", "updatedAt", "closedAt", "isMine", "myApplication", "stats"}
	summaryKeys     = []string{"id", "kind", "title", "status", "author"}
	applicationKeys = []string{"id", "listing", "applicant", "message", "status", "createdAt", "updatedAt", "decidedAt"}
	statsKeys       = []string{"pending", "accepted", "declined", "total"}
)

// expectUser checks the shape of a User.
func expectUser(t *testing.T, u J) {
	t.Helper()
	expectKeys(t, "user", u, userKeys...)
	expectTimestamp(t, u, "createdAt")
	if _, ok := u["skills"].([]any); !ok {
		t.Errorf("user.skills = %v, want array", u["skills"])
	}
}

// expectListing checks the shape of a Listing (nested author, application, stats).
func expectListing(t *testing.T, l J) {
	t.Helper()
	expectKeys(t, "listing", l, listingKeys...)
	expectUser(t, obj(t, l, "author"))
	expectTimestamp(t, l, "createdAt")
	expectTimestamp(t, l, "updatedAt")
	if l["closedAt"] != nil {
		expectTimestamp(t, l, "closedAt")
	}
	if l["myApplication"] != nil {
		expectApplication(t, obj(t, l, "myApplication"))
	}
	if l["stats"] != nil {
		expectKeys(t, "stats", obj(t, l, "stats"), statsKeys...)
	}
	if _, ok := l["isMine"].(bool); !ok {
		t.Errorf("isMine = %v, want bool", l["isMine"])
	}
	if _, ok := l["tags"].([]any); !ok {
		t.Errorf("tags = %v, want array", l["tags"])
	}
}

// expectApplication checks the shape of an Application.
func expectApplication(t *testing.T, a J) {
	t.Helper()
	expectKeys(t, "application", a, applicationKeys...)
	s := obj(t, a, "listing")
	expectKeys(t, "listing summary", s, summaryKeys...)
	expectUser(t, obj(t, s, "author"))
	expectUser(t, obj(t, a, "applicant"))
	expectTimestamp(t, a, "createdAt")
	expectTimestamp(t, a, "updatedAt")
	if a["decidedAt"] != nil {
		expectTimestamp(t, a, "decidedAt")
	}
}

// expectTimestamp checks an RFC 3339 UTC timestamp.
func expectTimestamp(t *testing.T, m J, key string) {
	t.Helper()
	s, ok := m[key].(string)
	if !ok {
		t.Errorf("%s = %v, want RFC 3339 string", key, m[key])
		return
	}
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil || !strings.HasSuffix(s, "Z") {
		t.Errorf("%s = %q, want RFC 3339 in UTC", key, s)
		return
	}
	if time.Since(ts) > 24*time.Hour || time.Until(ts) > time.Minute {
		t.Errorf("%s = %s is not recent", key, s)
	}
}

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

// path builds a path with fmt.Sprintf.
func path(format string, args ...any) string { return fmt.Sprintf(format, args...) }
