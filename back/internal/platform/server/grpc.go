package server

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
)

// MsgInternal is the user-facing message for internal errors (SPEC §4.2).
const MsgInternal = "Внутренняя ошибка"

// Metadata keys carrying HTTP request data over the gRPC hop.
const (
	requestIDKey = "x-request-id"
	clientIPKey  = "x-client-ip"
)

// newGRPCServer builds the server with the otelgrpc stats handler and
// ChainUnaryInterceptor(recovery, prometheus, logging, auth, callUser). otelgrpc
// tracing is a stats handler (otelgrpc has no interceptors any more); it wraps the
// whole chain. Logging stays before auth, so rejected calls are logged too;
// callUser reports the authenticated user back to the logging interceptor.
func (s *Server) newGRPCServer(verifier auth.Verifier, publicMethods map[string]bool) *grpc.Server {
	opts := append(s.tel.GRPCServerOptions(),
		grpc.MaxRecvMsgSize(MaxMessageSize),
		grpc.MaxSendMsgSize(MaxMessageSize),
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandlerContext(recoveryHandler)),
			s.tel.GRPCUnaryInterceptor(),
			loggingInterceptor(s.baseLog),
			auth.UnaryInterceptor(verifier, publicMethods),
			callUserInterceptor,
		),
	)
	return grpc.NewServer(opts...)
}

func recoveryHandler(ctx context.Context, p any) error {
	logger.FromContext(ctx).Error("grpc panic",
		zap.Any("panic", p),
		zap.ByteString("stack", debug.Stack()),
	)
	return status.Error(codes.Internal, MsgInternal)
}

// callInfo is filled by interceptors further down the chain for the call log line.
type callInfo struct {
	userID string
}

type callInfoKey struct{}

// callUserInterceptor runs after auth and records the authenticated user id in
// the callInfo of the logging interceptor.
func callUserInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if ci, ok := ctx.Value(callInfoKey{}).(*callInfo); ok {
		if id, ok := auth.UserIDFromContext(ctx); ok {
			ci.userID = id.String()
		}
	}
	return handler(ctx, req)
}

// loggingInterceptor puts the base logger and the request id (from x-request-id
// metadata set by the gateway) into the context and logs every call with method,
// code, duration, peer and user_id (when the call is authenticated).
func loggingInterceptor(base *zap.Logger) grpc.UnaryServerInterceptor {
	base = base.With(zap.String("component", "grpc"))
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		var clientIP string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if v := md.Get(requestIDKey); len(v) > 0 {
				ctx = logger.WithRequestID(ctx, v[0])
			}
			if v := md.Get(clientIPKey); len(v) > 0 {
				clientIP = v[0]
			}
		}
		ctx = logger.WithContext(ctx, base)
		ci := &callInfo{}

		resp, err := handler(context.WithValue(ctx, callInfoKey{}, ci), req)

		code := status.Code(err)
		fields := []zap.Field{
			zap.String("grpc.method", info.FullMethod),
			zap.String("grpc.code", code.String()),
			zap.Duration("duration", time.Since(start)),
		}
		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			fields = append(fields, zap.String("peer", p.Addr.String()))
		}
		if clientIP != "" {
			fields = append(fields, zap.String("client_ip", clientIP))
		}
		l := logger.FromContext(ctx)
		if ci.userID != "" {
			l = l.With(zap.String(logger.KeyUserID, ci.userID))
		}
		if isServerError(code) {
			l.Error("grpc call", append(fields, zap.Error(err))...)
		} else {
			l.Info("grpc call", fields...)
		}
		return resp, err
	}
}

func isServerError(c codes.Code) bool {
	switch c {
	case codes.Internal, codes.Unknown, codes.DataLoss, codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}
