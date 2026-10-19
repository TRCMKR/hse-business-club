package auth

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
)

const bearerPrefix = "bearer "

// UnaryInterceptor authenticates calls by the "authorization: Bearer <jwt>" metadata.
// publicMethods holds full method names (e.g. "/hbc.v1.HseBusinessClub/V1Ping") that
// do not require a token; for them a valid token is still attached to the context,
// an invalid one is ignored. Other methods get Unauthenticated «Требуется вход».
func UnaryInterceptor(v Verifier, publicMethods map[string]bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		token := tokenFromMetadata(ctx)
		public := publicMethods[info.FullMethod]

		if token == "" {
			if public {
				return handler(ctx, req)
			}
			return nil, status.Error(codes.Unauthenticated, MsgUnauthenticated)
		}

		id, err := v.Verify(token)
		if err != nil {
			if public {
				return handler(ctx, req)
			}
			logger.FromContext(ctx).Debug("token rejected", zap.Error(err))
			return nil, status.Error(codes.Unauthenticated, MsgUnauthenticated)
		}

		ctx = WithUserID(ctx, id)
		ctx = logger.WithUserID(ctx, id.String())
		return handler(ctx, req)
	}
}

func tokenFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, v := range md.Get("authorization") {
		if len(v) > len(bearerPrefix) && strings.EqualFold(v[:len(bearerPrefix)], bearerPrefix) {
			return strings.TrimSpace(v[len(bearerPrefix):])
		}
	}
	return ""
}
