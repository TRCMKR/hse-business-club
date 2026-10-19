package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func do(t *testing.T, method, url, token, body string) (int, http.Header, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

// Request parsing errors of the gateway get the Russian text of SPEC §2.4.
func TestRequestParseErrors(t *testing.T) {
	e := start(t, nil)
	tok, err := e.jwt.Issue([16]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"code":3,"message":"Некорректный формат запроса","details":[]}`

	cases := []struct {
		name, method, path, token, body string
	}{
		{"invalid json", http.MethodPost, "/v1/auth/register", "", `{"username":`},
		{"not an object", http.MethodPost, "/v1/auth/login", "", `[1,2]`},
		{"wrong field type", http.MethodPost, "/v1/auth/login", "", `{"username":1}`},
		{"unknown enum in body", http.MethodPost, "/v1/listings", tok, `{"kind":"LISTING_KIND_NOPE"}`},
		{"unknown stage in body", http.MethodPost, "/v1/listings", tok, `{"kind":"LISTING_KIND_STARTUP","stage":"MVP"}`},
		{"unknown enum in update", http.MethodPut, "/v1/listings/x", tok, `{"stage":"nope"}`},
		{"unknown enum in query", http.MethodGet, "/v1/listings?kind=NOPE", tok, ""},
		{"bad number in query", http.MethodGet, "/v1/listings?page=abc", tok, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, _, body := do(t, c.method, e.public.URL+c.path, c.token, c.body)
			if code != http.StatusBadRequest || !jsonEqual(t, body, want) {
				t.Fatalf("%s %s = %d %s, want 400 %s", c.method, c.path, code, body, want)
			}
		})
	}

	// An empty body is not a parsing error: the call reaches the service.
	if code, _, body := do(t, http.MethodPost, e.public.URL+"/v1/auth/register", tok, ""); code != http.StatusNotImplemented {
		t.Fatalf("empty body = %d %s, want 501 from the stub", code, body)
	}
	// Unknown fields are discarded, not rejected.
	if code, _, body := do(t, http.MethodPost, e.public.URL+"/v1/auth/login", tok, `{"extra":true}`); code != http.StatusNotImplemented {
		t.Fatalf("unknown field = %d %s, want 501 from the stub", code, body)
	}
	// Known enum names (snake_case keys too) and enum numbers pass.
	for _, body := range []string{`{"kind":"LISTING_KIND_STARTUP","stage":"STARTUP_STAGE_MVP"}`, `{"kind":1,"stage":3}`, `{"looking_for":"x","kind":"LISTING_KIND_VACANCY"}`} {
		if code, _, resp := do(t, http.MethodPost, e.public.URL+"/v1/listings", tok, body); code != http.StatusNotImplemented {
			t.Fatalf("POST /v1/listings %s = %d %s, want 501 from the stub", body, code, resp)
		}
	}
}

// 401 carries an ASCII challenge, not the Russian message.
func TestUnauthenticatedChallenge(t *testing.T) {
	e := start(t, nil)

	code, h, _ := do(t, http.MethodGet, e.public.URL+"/v1/me", "", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("GET /v1/me = %d", code)
	}
	if got := h.Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
	}
}

// The debug port proxies /v1/* with the same route label as the public port.
func TestDebugProxyRouteLabel(t *testing.T) {
	e := start(t, nil)

	if code, _, body := do(t, http.MethodGet, e.debug.URL+"/v1/ping", "", ""); code != http.StatusOK {
		t.Fatalf("debug /v1/ping = %d %s", code, body)
	}
	_, _, metrics := do(t, http.MethodGet, e.debug.URL+"/metrics", "", "")
	if !strings.Contains(metrics, `http_requests_total{method="GET",route="/v1/ping",status="200"} 1`) {
		t.Fatalf("metrics have no route=/v1/ping sample:\n%s", grepLines(metrics, "http_requests_total"))
	}
	if strings.Contains(metrics, "/v1/v1/") {
		t.Fatalf("metrics contain a doubled route:\n%s", grepLines(metrics, "http_requests_total"))
	}
}

// The "grpc call" log line carries user_id of an authenticated call.
func TestCallLogHasUserID(t *testing.T) {
	e := start(t, nil)
	id := [16]byte{0xab}
	tok, err := e.jwt.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	do(t, http.MethodGet, e.public.URL+"/v1/me", tok, "")
	do(t, http.MethodGet, e.public.URL+"/v1/ping", "", "")

	var withUser, anonymous bool
	for _, line := range strings.Split(e.logs.String(), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil || rec["message"] != "grpc call" {
			continue
		}
		switch rec["grpc.method"] {
		case "/hbc.v1.HseBusinessClub/V1GetMe":
			withUser = rec["user_id"] == "ab000000-0000-0000-0000-000000000000"
		case "/hbc.v1.HseBusinessClub/V1Ping":
			_, has := rec["user_id"]
			anonymous = !has
		}
	}
	if !withUser || !anonymous {
		t.Fatalf("want user_id on V1GetMe and none on V1Ping, logs:\n%s", e.logs.String())
	}
}

func grepLines(s, sub string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
