package bff

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
	"github.com/trcmkr/hse-business-club/back/internal/platform/server"
	"github.com/trcmkr/hse-business-club/back/internal/platform/telemetry"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// gatewayEnv is the BFF on fakes behind the real server: chi → grpc-gateway → gRPC hop.
type gatewayEnv struct {
	*env
	srv    *server.Server
	public *httptest.Server
	jwt    *auth.JWT
}

func startGateway(t *testing.T) *gatewayEnv {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	log, err := logger.NewWithWriter("error", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	tel, err := telemetry.Init(ctx, config.Telemetry{ServiceName: "bff-test"})
	if err != nil {
		t.Fatal(err)
	}
	jwt := auth.NewJWT("bff-test-secret-bff-test-secret-xx", time.Hour)
	e := newEnv(t)
	e.bff.Issuer = jwt
	e.bff.AccessTTL = jwt.TTL()

	const anyPort = "127.0.0.1:0" // 8080–8082 may be taken on the machine
	srv, err := server.New(ctx, config.Server{
		HTTPAddr: anyPort, DebugAddr: anyPort, GRPCAddr: anyPort, ShutdownTimeout: 5 * time.Second,
	}, log, tel, jwt, e.bff)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	g := &gatewayEnv{env: e, srv: srv, public: httptest.NewServer(srv.PublicHandler()), jwt: jwt}
	t.Cleanup(func() {
		g.public.Close()
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
	return g
}

func (g *gatewayEnv) do(t *testing.T, method, path, token string, body string) (*http.Response, []byte) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, g.public.URL+path, r)
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
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, data
}

func decodeJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("invalid JSON %q: %v", data, err)
	}
	return v
}

// errorJSON is the decoded grpc-gateway error body (SPEC §4.2).
func errorJSON(code codes.Code, msg string) map[string]any {
	return map[string]any{"code": float64(code), "message": msg, "details": []any{}}
}

func TestGatewayAvatar(t *testing.T) {
	t.Parallel()
	g := startGateway(t)
	anna := g.user(t, nameAnna)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xff, 0xd9}
	u, err := g.users.SetAvatar(context.Background(), anna.ID, jpeg)
	if err != nil {
		t.Fatal(err)
	}

	resp, body := g.do(t, http.MethodGet, "/v1/avatars/"+u.AvatarKey, "", "") // public: no token
	if resp.StatusCode != http.StatusOK || !bytes.Equal(body, jpeg) {
		t.Fatalf("GET avatar = %d %q", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != jpegType {
		t.Fatalf("Content-Type = %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", cc)
	}

	resp, body = g.do(t, http.MethodGet, "/v1/avatars/00000000-0000-4000-8000-000000000000.jpg", "", "")
	if resp.StatusCode != http.StatusNotFound || !reflect.DeepEqual(decodeJSON(t, body), errorJSON(codes.NotFound, fmsgAvatarNotFound)) {
		t.Fatalf("GET unknown avatar = %d %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unknown avatar Content-Type = %q", ct)
	}
	if resp.Header.Get("Cache-Control") == "immutable" {
		t.Fatal("error response must not be cached as immutable")
	}
}

// TestGatewayJSON checks the protojson shape of composed responses end to end:
// avatarUrl, null closedAt/stats/myApplication, enum names, error format.
func TestGatewayJSON(t *testing.T) {
	t.Parallel()
	g := startGateway(t)

	resp, body := g.do(t, http.MethodPost, "/v1/auth/register", "", `{"username":"boris","password":"demo1234","displayName":"Борис"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register = %d %s", resp.StatusCode, body)
	}
	reg := decodeJSON(t, body)
	token, _ := reg["accessToken"].(string)
	if _, err := g.jwt.Verify(token); err != nil {
		t.Fatalf("token from register is invalid: %v", err)
	}
	refresh, _ := reg["refreshToken"].(string)
	if refresh == "" || reg["accessExpiresIn"] != float64(3600) || reg["user"] == nil || len(reg) != 4 {
		t.Fatalf("register = %s", body)
	}

	// Refresh and logout are public: no Authorization header.
	resp, body = g.do(t, http.MethodPost, "/v1/auth/refresh", "", `{"refreshToken":"`+refresh+`"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh = %d %s", resp.StatusCode, body)
	}
	next, _ := decodeJSON(t, body)["refreshToken"].(string)
	resp, body = g.do(t, http.MethodPost, "/v1/auth/logout", "", `{"refreshToken":"`+next+`"}`)
	if resp.StatusCode != http.StatusOK || string(body) != "{}" {
		t.Fatalf("logout = %d %s", resp.StatusCode, body)
	}
	resp, body = g.do(t, http.MethodPost, "/v1/auth/refresh", "", `{"refreshToken":"`+next+`"}`)
	if resp.StatusCode != http.StatusUnauthorized || !reflect.DeepEqual(decodeJSON(t, body), errorJSON(codes.Unauthenticated, msgSessionExpired)) {
		t.Fatalf("refresh after logout = %d %s", resp.StatusCode, body)
	}

	anna := g.user(t, nameAnna)
	fin := g.startup(t, anna.ID, "FinTrack")
	if _, err := g.users.SetAvatar(context.Background(), anna.ID, []byte("jpeg")); err != nil {
		t.Fatal(err)
	}

	resp, body = g.do(t, http.MethodGet, "/v1/listings?kind=LISTING_KIND_STARTUP", token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("feed = %d %s", resp.StatusCode, body)
	}
	feed := decodeJSON(t, body)
	items, _ := feed["items"].([]any)
	if len(items) != 1 || feed["total"] != float64(1) || feed["page"] != float64(1) || feed["limit"] != float64(20) {
		t.Fatalf("feed = %s", body)
	}
	item, _ := items[0].(map[string]any)
	for key, want := range map[string]any{
		"id": fin.ID.String(), "kind": "LISTING_KIND_STARTUP", "stage": "STARTUP_STAGE_MVP",
		"status": "LISTING_STATUS_ACTIVE", "isMine": false, "closedAt": nil, "stats": nil, "myApplication": nil,
		"compensation": "",
	} {
		if got, ok := item[key]; !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("item[%q] = %#v (present %v), want %#v", key, got, ok, want)
		}
	}
	author, _ := item["author"].(map[string]any)
	if url, _ := author["avatarUrl"].(string); !strings.HasPrefix(url, "/v1/avatars/") {
		t.Errorf("author.avatarUrl = %q", url)
	}

	resp, body = g.do(t, http.MethodPost, "/v1/listings/"+fin.ID.String()+"/applications", token, `{"message":"Хочу"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("apply = %d %s", resp.StatusCode, body)
	}
	resp, body = g.do(t, http.MethodPost, "/v1/listings/"+fin.ID.String()+"/applications", token, `{"message":"Хочу"}`)
	if resp.StatusCode != http.StatusConflict || !reflect.DeepEqual(decodeJSON(t, body), errorJSON(codes.AlreadyExists, fmsgAlreadyApplied)) {
		t.Fatalf("repeated apply = %d %s", resp.StatusCode, body)
	}

	resp, body = g.do(t, http.MethodPost, "/v1/listings", token, `{"title":"Без вида"}`)
	if resp.StatusCode != http.StatusBadRequest || !reflect.DeepEqual(decodeJSON(t, body), errorJSON(codes.InvalidArgument, msgKindRequired)) {
		t.Fatalf("create without kind = %d %s", resp.StatusCode, body)
	}
}

// TestAllMethodsImplemented calls every RPC of the service over gRPC and checks that
// none of them answers Unimplemented.
func TestAllMethodsImplemented(t *testing.T) {
	t.Parallel()
	g := startGateway(t)
	me := g.user(t, nameAnna)
	token, err := g.jwt.Issue(me.ID)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := grpc.NewClient(g.srv.GRPCAddr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)

	desc := hbcv1.HseBusinessClub_ServiceDesc
	if len(desc.Methods) != 24 {
		t.Fatalf("service has %d methods, want 24", len(desc.Methods))
	}
	for _, m := range desc.Methods {
		err := conn.Invoke(ctx, "/"+desc.ServiceName+"/"+m.MethodName, &emptypb.Empty{}, &emptypb.Empty{})
		if code := status.Code(err); code == codes.Unimplemented || code == codes.Internal {
			t.Errorf("%s: %v", m.MethodName, err)
		}
	}
}
