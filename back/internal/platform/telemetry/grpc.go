package telemetry

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
)

// GRPCServerOptions returns server options with the otelgrpc stats handler.
// (otelgrpc no longer ships interceptors; tracing of the gRPC server is a stats handler.)
func (t *Telemetry) GRPCServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler(otelgrpc.WithTracerProvider(t.tp))),
	}
}

// GRPCClientOptions returns dial options with the otelgrpc client stats handler
// (used by the gateway connection).
func (t *Telemetry) GRPCClientOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(otelgrpc.WithTracerProvider(t.tp))),
	}
}

// GRPCMetrics returns the Prometheus gRPC server metrics (grpc_server_handled_total, ...).
func (t *Telemetry) GRPCMetrics() *grpcprom.ServerMetrics {
	return t.grpc
}

// GRPCUnaryInterceptor returns the Prometheus gRPC server interceptor.
func (t *Telemetry) GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	return t.grpc.UnaryServerInterceptor()
}

// InitGRPCMetrics pre-populates metrics with zero values for every registered method.
func (t *Telemetry) InitGRPCMetrics(s reflection.ServiceInfoProvider) {
	t.grpc.InitializeMetrics(s)
}
