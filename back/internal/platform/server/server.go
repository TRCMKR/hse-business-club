// Package server runs the gRPC server, the grpc-gateway reverse proxy behind a chi
// router on HTTP_ADDR, and the debug router (metrics, pprof, docs) on DEBUG_ADDR.
//
// The gateway always goes through a real gRPC hop (ARCHITECTURE §3):
// gRPC server on GRPC_ADDR → grpc.NewClient(GRPC_ADDR) → RegisterGateway(mux, conn)
// → mux mounted into chi.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/platform/config"
	"github.com/trcmkr/hse-business-club/back/internal/platform/telemetry"
)

// MaxMessageSize is the gRPC message limit on the server and the gateway client:
// a 5 MB photo is ~6.8 MB as base64 JSON.
const MaxMessageSize = 16 << 20

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	checkTimeout      = time.Second
)

// Service is a gRPC service exposed through the gateway.
type Service interface {
	RegisterGRPC(grpc.ServiceRegistrar)
	RegisterGateway(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error
}

// PublicMethodsProvider is optionally implemented by a Service to list full gRPC
// method names that do not require authentication.
type PublicMethodsProvider interface {
	PublicMethods() []string
}

// SwaggerProvider is optionally implemented by a Service to expose its OpenAPI v2
// document on DEBUG_ADDR/docs/swagger.json. Its paths are also registered as chi
// routes, so HTTP metrics and spans are labelled by route template.
type SwaggerProvider interface {
	SwaggerJSON() []byte
}

// Check is a readiness check (for /readyz).
type Check func(ctx context.Context) error

type namedCheck struct {
	name string
	fn   Check
}

// Server owns the listeners, the gRPC server, the gateway connection and both routers.
type Server struct {
	cfg config.Server
	log *zap.Logger
	tel *telemetry.Telemetry

	grpcLis, httpLis, debugLis net.Listener

	grpcServer *grpc.Server
	conn       *grpc.ClientConn
	gateway    *runtime.ServeMux
	public     chi.Router
	debug      chi.Router
	swagger    []byte

	mu     sync.RWMutex
	checks []namedCheck
}

// New listens on cfg.GRPCAddr, cfg.HTTPAddr and cfg.DebugAddr, registers services
// on the gRPC server and the gateway and builds the routers. Call Run to serve.
func New(ctx context.Context, cfg config.Server, log *zap.Logger, tel *telemetry.Telemetry, verifier auth.Verifier, services ...Service) (_ *Server, err error) {
	s := &Server{cfg: cfg, log: log.With(zap.String("component", "server")), tel: tel}
	defer func() {
		if err != nil {
			s.closeAll()
		}
	}()

	if s.grpcLis, err = net.Listen("tcp", cfg.GRPCAddr); err != nil {
		return nil, fmt.Errorf("server: listen grpc %s: %w", cfg.GRPCAddr, err)
	}
	if s.httpLis, err = net.Listen("tcp", cfg.HTTPAddr); err != nil {
		return nil, fmt.Errorf("server: listen http %s: %w", cfg.HTTPAddr, err)
	}
	if s.debugLis, err = net.Listen("tcp", cfg.DebugAddr); err != nil {
		return nil, fmt.Errorf("server: listen debug %s: %w", cfg.DebugAddr, err)
	}

	public := map[string]bool{}
	for _, svc := range services {
		if p, ok := svc.(PublicMethodsProvider); ok {
			for _, m := range p.PublicMethods() {
				public[m] = true
			}
		}
		if p, ok := svc.(SwaggerProvider); ok && s.swagger == nil {
			s.swagger = p.SwaggerJSON()
		}
	}

	s.grpcServer = s.newGRPCServer(verifier, public)
	for _, svc := range services {
		svc.RegisterGRPC(s.grpcServer)
	}
	tel.InitGRPCMetrics(s.grpcServer)

	s.conn, err = grpc.NewClient(dialTarget(s.grpcLis.Addr()), append(tel.GRPCClientOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(MaxMessageSize),
			grpc.MaxCallSendMsgSize(MaxMessageSize),
		),
	)...)
	if err != nil {
		return nil, fmt.Errorf("server: grpc client: %w", err)
	}

	s.gateway = newGatewayMux()
	for _, svc := range services {
		if err := svc.RegisterGateway(ctx, s.gateway, s.conn); err != nil {
			return nil, fmt.Errorf("server: register gateway: %w", err)
		}
	}

	if s.public, err = s.newPublicRouter(); err != nil {
		return nil, err
	}
	s.debug = s.newDebugRouter()
	return s, nil
}

// dialTarget turns a listener address into a dialable target (":8082" → "127.0.0.1:8082").
func dialTarget(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// AddReadinessCheck adds a named check to /readyz (e.g. "db", "blob").
// Checks run in parallel with a 1s timeout each; the response keeps insertion order.
func (s *Server) AddReadinessCheck(name string, fn Check) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks = append(s.checks, namedCheck{name: name, fn: fn})
}

// PublicHandler returns the public router (HTTP_ADDR).
func (s *Server) PublicHandler() http.Handler { return s.public }

// DebugHandler returns the debug router (DEBUG_ADDR).
func (s *Server) DebugHandler() http.Handler { return s.debug }

// HTTPAddr returns the bound public address.
func (s *Server) HTTPAddr() net.Addr { return s.httpLis.Addr() }

// DebugAddr returns the bound debug address.
func (s *Server) DebugAddr() net.Addr { return s.debugLis.Addr() }

// GRPCAddr returns the bound gRPC address.
func (s *Server) GRPCAddr() net.Addr { return s.grpcLis.Addr() }

// Run serves until ctx is cancelled or a listener fails, then shuts down gracefully:
// stop accepting HTTP (public, then debug) → gRPC GracefulStop bounded by
// SHUTDOWN_TIMEOUT (then Stop) → close the gateway connection. Flushing traces and
// closing the pool are up to the caller after Run returns.
func (s *Server) Run(ctx context.Context) error {
	publicSrv := newHTTPServer(s.public)
	debugSrv := newHTTPServer(s.debug)

	errCh := make(chan error, 3)
	go func() {
		if err := s.grpcServer.Serve(s.grpcLis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			errCh <- fmt.Errorf("server: grpc: %w", err)
		}
	}()
	go func() {
		if err := publicSrv.Serve(s.httpLis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("server: http: %w", err)
		}
	}()
	go func() {
		if err := debugSrv.Serve(s.debugLis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("server: debug: %w", err)
		}
	}()
	s.log.Info("server started",
		zap.Stringer("http_addr", s.HTTPAddr()),
		zap.Stringer("debug_addr", s.DebugAddr()),
		zap.Stringer("grpc_addr", s.GRPCAddr()),
	)

	var runErr error
	select {
	case <-ctx.Done():
		s.log.Info("shutdown requested")
	case runErr = <-errCh:
		s.log.Error("server failed", zap.Error(runErr))
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.ShutdownTimeout)
	defer cancel()

	if err := publicSrv.Shutdown(shutdownCtx); err != nil {
		s.log.Warn("http shutdown", zap.Error(err))
	}
	if err := debugSrv.Shutdown(shutdownCtx); err != nil {
		s.log.Warn("debug shutdown", zap.Error(err))
	}

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-shutdownCtx.Done():
		s.log.Warn("grpc graceful stop timed out, forcing")
		s.grpcServer.Stop()
		<-stopped
	}

	if err := s.conn.Close(); err != nil {
		s.log.Warn("grpc client close", zap.Error(err))
	}
	s.log.Info("server stopped")
	return runErr
}

func newHTTPServer(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// closeAll releases resources after a failed New.
func (s *Server) closeAll() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
	for _, l := range []net.Listener{s.grpcLis, s.httpLis, s.debugLis} {
		if l != nil {
			_ = l.Close()
		}
	}
}
