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
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
	"github.com/trcmkr/hse-business-club/back/internal/platform/server"
	"github.com/trcmkr/hse-business-club/back/internal/platform/telemetry"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

const (
	refreshNext = "refresh-next"
	jsonID      = "id"
	jsonStatus  = "status"
)

// serverT is the testing interface of mocks called from gRPC server goroutines.
// testing.T.FailNow must run on the test goroutine, and Goexit in a handler would
// leave the request without a response. Here FailNow panics instead: the recovery
// interceptor answers Internal, and the test fails with the mock's message (from
// Errorf) instead of hanging.
type serverT struct {
	*testing.T

	mu    sync.Mutex
	quiet bool // record mock errors without failing the test (tests serverT itself)
	errs  []string
}

// errMockFailed is the panic value of serverT.FailNow.
const errMockFailed = "mock: call failed, see the test log"

func (s *serverT) Errorf(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errs = append(s.errs, format)
	if !s.quiet {
		s.T.Errorf(format, args...)
	}
}

func (s *serverT) FailNow() { panic(errMockFailed) }

// gatewayEnv is the BFF on mocks behind the real server: chi → grpc-gateway → gRPC hop.
type gatewayEnv struct {
	*env
	st     *serverT
	srv    *server.Server
	public *httptest.Server
	jwt    *auth.JWT
	client *http.Client
}

func startGateway(t *testing.T) *gatewayEnv {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	log, err := logger.NewWithWriter("error", io.Discard)
	require.NoError(t, err)
	tel, err := telemetry.Init(ctx, config.Telemetry{ServiceName: "bff-test"})
	require.NoError(t, err)
	jwt := auth.NewJWT("bff-test-secret-bff-test-secret-xx", time.Hour)
	st := &serverT{T: t}
	e := newEnvOn(st)
	e.bff.Issuer = jwt
	e.bff.AccessTTL = jwt.TTL()

	const anyPort = "127.0.0.1:0" // 8080–8082 may be taken on the machine
	srv, err := server.New(ctx, config.Server{
		HTTPAddr: anyPort, DebugAddr: anyPort, GRPCAddr: anyPort, ShutdownTimeout: 5 * time.Second,
	}, log, tel, jwt, e.bff)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()

	g := &gatewayEnv{
		env: e, st: st, srv: srv, public: httptest.NewServer(srv.PublicHandler()), jwt: jwt,
		client: &http.Client{Timeout: 10 * time.Second}, // a lost response fails, not hangs
	}
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

func (g *gatewayEnv) token(t *testing.T, id uuid.UUID) string {
	t.Helper()
	token, err := g.jwt.Issue(id)
	require.NoError(t, err)
	return token
}

func (g *gatewayEnv) do(t *testing.T, method, path, token string, body string) (*http.Response, []byte) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, g.public.URL+path, r)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := g.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
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

// wantFields checks the given keys of a JSON object (present, with the value).
func wantFields(t *testing.T, obj map[string]any, want map[string]any) {
	t.Helper()
	for key, w := range want {
		if got, ok := obj[key]; !ok || !reflect.DeepEqual(got, w) {
			t.Errorf("[%q] = %#v (present %v), want %#v", key, got, ok, w)
		}
	}
}

func TestGatewayAvatar(t *testing.T) {
	t.Parallel()
	g := startGateway(t)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xff, 0xd9}
	const unknown = "00000000-0000-4000-8000-000000000000.jpg"
	g.users.EXPECT().OpenAvatar(mock.Anything, avatarKey).Return(&users.Avatar{ContentType: jpegType, Data: jpeg}, nil).Once()
	g.users.EXPECT().OpenAvatar(mock.Anything, unknown).Return(nil, errOf(apperr.KindNotFound, cmsgAvatarNotFound)).Once()

	resp, body := g.do(t, http.MethodGet, avatarPathPrefix+avatarKey, "", "") // public: no token
	if resp.StatusCode != http.StatusOK || !bytes.Equal(body, jpeg) {
		t.Fatalf("GET avatar = %d %q", resp.StatusCode, body)
	}
	require.Equal(t, jpegType, resp.Header.Get("Content-Type"))
	require.Equal(t, "public, max-age=31536000, immutable", resp.Header.Get("Cache-Control"))

	resp, body = g.do(t, http.MethodGet, avatarPathPrefix+unknown, "", "")
	if resp.StatusCode != http.StatusNotFound || !reflect.DeepEqual(decodeJSON(t, body), errorJSON(codes.NotFound, cmsgAvatarNotFound)) {
		t.Fatalf("GET unknown avatar = %d %s", resp.StatusCode, body)
	}
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	require.NotContains(t, resp.Header.Get("Cache-Control"), "immutable", "error response must not be cached as immutable")
}

// TestGatewayJSON checks the protojson shape of composed responses end to end:
// avatarUrl, null closedAt/stats/myApplication, enum names, error format.
func TestGatewayJSON(t *testing.T) {
	t.Parallel()
	g := startGateway(t)
	boris := newUser(nameBoris)
	boris.DisplayName = "Борис"
	anna := withAvatar(newUser(nameAnna), avatarKey)
	fin := newStartup(anna.ID, titleFinTrack)

	// Auth: the access token comes from the real JWT issuer, the refresh token from
	// the users context.
	g.users.EXPECT().Register(mock.Anything, users.RegisterParams{Username: nameBoris, Password: testPassword, DisplayName: "Борис"}).
		Return(boris, nil).Once()
	g.users.EXPECT().IssueRefresh(mock.Anything, boris.ID).Return(refreshToken(testRefresh), nil).Once()
	g.users.EXPECT().RotateRefresh(mock.Anything, testRefresh).Return(boris.ID, refreshToken(refreshNext), nil).Once()
	g.users.EXPECT().Get(mock.Anything, boris.ID).Return(boris, nil).Once()
	g.users.EXPECT().RevokeRefresh(mock.Anything, refreshNext).Return(nil).Once()
	g.users.EXPECT().RotateRefresh(mock.Anything, refreshNext).
		Return(uuid.Nil, nil, errOf(apperr.KindUnauthenticated, msgSessionExpired)).Once()

	resp, body := g.do(t, http.MethodPost, "/v1/auth/register", "", `{"username":"boris","password":"demo1234","displayName":"Борис"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register = %d %s", resp.StatusCode, body)
	}
	reg := decodeJSON(t, body)
	token, _ := reg["accessToken"].(string)
	id, err := g.jwt.Verify(token)
	require.NoError(t, err, "token from register is invalid")
	require.Equal(t, boris.ID, id)
	if reg["refreshToken"] != testRefresh || reg["accessExpiresIn"] != float64(3600) || reg["user"] == nil || len(reg) != 4 {
		t.Fatalf("register = %s", body)
	}

	// Refresh and logout are public: no Authorization header.
	resp, body = g.do(t, http.MethodPost, "/v1/auth/refresh", "", `{"refreshToken":"`+testRefresh+`"}`)
	if resp.StatusCode != http.StatusOK || decodeJSON(t, body)["refreshToken"] != refreshNext {
		t.Fatalf("refresh = %d %s", resp.StatusCode, body)
	}
	resp, body = g.do(t, http.MethodPost, "/v1/auth/logout", "", `{"refreshToken":"`+refreshNext+`"}`)
	if resp.StatusCode != http.StatusOK || string(body) != "{}" {
		t.Fatalf("logout = %d %s", resp.StatusCode, body)
	}
	resp, body = g.do(t, http.MethodPost, "/v1/auth/refresh", "", `{"refreshToken":"`+refreshNext+`"}`)
	if resp.StatusCode != http.StatusUnauthorized || !reflect.DeepEqual(decodeJSON(t, body), errorJSON(codes.Unauthenticated, msgSessionExpired)) {
		t.Fatalf("refresh after logout = %d %s", resp.StatusCode, body)
	}

	// Feed of a foreign startup.
	f := feedFilter(listings.KindStartup)
	g.listings.EXPECT().ListActive(mock.Anything, f).Return(listingPage(f, 1, fin), nil).Once()
	g.expectUsers(2, anna, boris) // the feed and the application below
	g.apps.EXPECT().GetMineForListings(mock.Anything, boris.ID, []uuid.UUID{fin.ID}).
		Return(map[uuid.UUID]*applications.Application{}, nil).Once()

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
	wantFields(t, item, map[string]any{
		jsonID: fin.ID.String(), "kind": "LISTING_KIND_STARTUP", "stage": "STARTUP_STAGE_MVP",
		jsonStatus: "LISTING_STATUS_ACTIVE", "isMine": false, "closedAt": nil, "stats": nil, "myApplication": nil,
		"compensation": "",
	})
	author, _ := item["author"].(map[string]any)
	require.Equal(t, avatarPathPrefix+avatarKey, author["avatarUrl"])

	// Apply, then the context rejects the repeated application.
	app := newApp(fin, boris.ID)
	app.Message = "Хочу"
	g.apps.EXPECT().Apply(mock.Anything, fin.ID, boris.ID, "Хочу").Return(app, nil).Once()
	g.apps.EXPECT().Apply(mock.Anything, fin.ID, boris.ID, "Хочу").Return(nil, errOf(apperr.KindConflict, cmsgAlreadyApplied)).Once()
	g.expectListings(1, fin)

	applyPath := "/v1/listings/" + fin.ID.String() + "/applications"
	resp, body = g.do(t, http.MethodPost, applyPath, token, `{"message":"Хочу"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("apply = %d %s", resp.StatusCode, body)
	}
	got := decodeJSON(t, body)
	wantFields(t, got, map[string]any{jsonID: app.ID.String(), jsonStatus: "APPLICATION_STATUS_PENDING", "decidedAt": nil, "message": "Хочу"})
	listing, _ := got["listing"].(map[string]any)
	wantFields(t, listing, map[string]any{jsonID: fin.ID.String(), "kind": "LISTING_KIND_STARTUP", jsonStatus: "LISTING_STATUS_ACTIVE"})

	resp, body = g.do(t, http.MethodPost, applyPath, token, `{"message":"Хочу"}`)
	if resp.StatusCode != http.StatusConflict || !reflect.DeepEqual(decodeJSON(t, body), errorJSON(codes.AlreadyExists, cmsgAlreadyApplied)) {
		t.Fatalf("repeated apply = %d %s", resp.StatusCode, body)
	}

	// A shape error: no port calls.
	resp, body = g.do(t, http.MethodPost, "/v1/listings", token, `{"title":"Без вида"}`)
	if resp.StatusCode != http.StatusBadRequest || !reflect.DeepEqual(decodeJSON(t, body), errorJSON(codes.InvalidArgument, msgKindRequired)) {
		t.Fatalf("create without kind = %d %s", resp.StatusCode, body)
	}
}

// TestGatewayUnexpectedPortCall: a port call without an expectation is reported as
// a mock failure and answered 500; the server keeps serving.
func TestGatewayUnexpectedPortCall(t *testing.T) {
	t.Parallel()
	g := startGateway(t)
	g.st.mu.Lock()
	g.st.quiet = true
	g.st.mu.Unlock()
	me := newUser(nameAnna)

	resp, body := g.do(t, http.MethodGet, "/v1/me", g.token(t, me.ID), "") // no Users.Get expectation
	if resp.StatusCode != http.StatusInternalServerError || !reflect.DeepEqual(decodeJSON(t, body), errorJSON(codes.Internal, server.MsgInternal)) {
		t.Fatalf("unexpected call = %d %s", resp.StatusCode, body)
	}
	g.st.mu.Lock()
	errs := strings.Join(g.st.errs, "\n")
	g.st.mu.Unlock()
	require.Contains(t, errs, "mock: I don't know what to return because the method call was unexpected")

	resp, body = g.do(t, http.MethodGet, "/v1/ping", "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode, "ping after a mock failure: %s", body)
}

// TestAllMethodsImplemented calls every RPC of the service over gRPC with an empty
// request and checks that none of them answers Unimplemented or Internal. RPCs that
// pass the shape checks reach the ports exactly once with zero values.
func TestAllMethodsImplemented(t *testing.T) {
	t.Parallel()
	g := startGateway(t)
	me := newUser(nameAnna)

	g.users.EXPECT().Register(mock.Anything, users.RegisterParams{}).Return(nil, apperr.Validation("username", cmsgUsername)).Once()
	g.users.EXPECT().Authenticate(mock.Anything, "", "").Return(nil, errOf(apperr.KindUnauthenticated, cmsgBadLogin)).Once()
	g.users.EXPECT().OpenAvatar(mock.Anything, "").Return(nil, errOf(apperr.KindNotFound, cmsgAvatarNotFound)).Once()
	g.users.EXPECT().Get(mock.Anything, me.ID).Return(me, nil).Once()
	g.users.EXPECT().UpdateProfile(mock.Anything, me.ID, users.UpdateProfileParams{}).
		Return(nil, apperr.Validation("display_name", cmsgDisplayName)).Once()
	g.users.EXPECT().RemoveAvatar(mock.Anything, me.ID).Return(me, nil).Once()
	f := feedFilter("")
	g.listings.EXPECT().ListActive(mock.Anything, f).Return(listingPage(f, 0), nil).Once()
	g.listings.EXPECT().ListByAuthor(mock.Anything, me.ID, listings.Status("")).Return(nil, nil).Once()
	g.apps.EXPECT().ListByApplicant(mock.Anything, me.ID, applications.Status("")).Return(nil, nil).Once()
	g.apps.EXPECT().ListReceived(mock.Anything, me.ID, applications.Status("")).Return(nil, nil).Once()

	conn, err := grpc.NewClient(g.srv.GRPCAddr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+g.token(t, me.ID))

	desc := hbcv1.HseBusinessClub_ServiceDesc
	require.Len(t, desc.Methods, 24)
	for _, m := range desc.Methods {
		err := conn.Invoke(ctx, "/"+desc.ServiceName+"/"+m.MethodName, &emptypb.Empty{}, &emptypb.Empty{})
		if code := status.Code(err); code == codes.Unimplemented || code == codes.Internal {
			t.Errorf("%s: %v", m.MethodName, err)
		}
	}
}
