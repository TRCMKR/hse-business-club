package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// Routing error messages (the gateway's own errors, before any gRPC call).
const (
	msgNotFound         = "Не найдено"
	msgMethodNotAllowed = "Метод не поддерживается"
)

// passThroughHeaders are gRPC response headers forwarded to HTTP as is
// (without the Grpc-Metadata- prefix). The BFF sets them with grpc.SetHeader.
var passThroughHeaders = map[string]bool{
	"cache-control": true,
	"etag":          true,
}

func newGatewayMux() *runtime.ServeMux {
	return runtime.NewServeMux(
		// HTTPBodyMarshaler serves google.api.HttpBody (V1GetAvatar) as raw bytes
		// with its Content-Type; everything else is protojson.
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.HTTPBodyMarshaler{
			Marshaler: &runtime.JSONPb{
				MarshalOptions:   protojson.MarshalOptions{EmitUnpopulated: true},
				UnmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
			},
		}),
		runtime.WithIncomingHeaderMatcher(incomingHeaderMatcher),
		runtime.WithOutgoingHeaderMatcher(outgoingHeaderMatcher),
		runtime.WithMetadata(requestIDMetadata),
		runtime.WithErrorHandler(errorHandler),
		runtime.WithRoutingErrorHandler(routingErrorHandler),
	)
}

func incomingHeaderMatcher(key string) (string, bool) {
	switch strings.ToLower(key) {
	case "authorization":
		return "authorization", true
	default:
		return runtime.DefaultHeaderMatcher(key)
	}
}

func outgoingHeaderMatcher(key string) (string, bool) {
	k := strings.ToLower(key)
	if passThroughHeaders[k] {
		return k, true
	}
	return runtime.MetadataHeaderPrefix + key, true
}

// requestIDMetadata forwards the chi request id and the client IP to the gRPC
// server for logs.
func requestIDMetadata(ctx context.Context, _ *http.Request) metadata.MD {
	md := metadata.MD{}
	if id := middleware.GetReqID(ctx); id != "" {
		md.Set(requestIDKey, id)
	}
	if ip := middleware.GetClientIP(ctx); ip != "" {
		md.Set(clientIPKey, ip)
	}
	return md
}

// errorHandler is DefaultHTTPErrorHandler with server-side error texts replaced by
// «Внутренняя ошибка» (the details are logged by the gRPC side).
func errorHandler(ctx context.Context, mux *runtime.ServeMux, m runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	st := status.Convert(err)
	if isServerError(st.Code()) {
		err = status.Error(st.Code(), MsgInternal)
	}
	runtime.DefaultHTTPErrorHandler(ctx, mux, m, w, r, err)
}

func routingErrorHandler(ctx context.Context, mux *runtime.ServeMux, m runtime.Marshaler, w http.ResponseWriter, r *http.Request, httpStatus int) {
	var err error
	switch httpStatus {
	case http.StatusMethodNotAllowed:
		err = &runtime.HTTPStatusError{HTTPStatus: httpStatus, Err: status.Error(codes.Unimplemented, msgMethodNotAllowed)}
	case http.StatusNotFound:
		err = &runtime.HTTPStatusError{HTTPStatus: httpStatus, Err: status.Error(codes.NotFound, msgNotFound)}
	default:
		runtime.DefaultRoutingErrorHandler(ctx, mux, m, w, r, httpStatus)
		return
	}
	runtime.DefaultHTTPErrorHandler(ctx, mux, m, w, r, err)
}
