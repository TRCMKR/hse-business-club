package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Routing error messages (the gateway's own errors, before any gRPC call).
const (
	msgNotFound         = "Не найдено"
	msgMethodNotAllowed = "Метод не поддерживается"
)

// MsgBadRequest replaces the English protojson/strconv text of request parsing
// errors (invalid JSON, unknown enum name, bad number in a query): SPEC §2.4.
const MsgBadRequest = "Некорректный формат запроса"

// badRequestError carries a parsing error. The generated gateway code turns it
// into InvalidArgument with "%v", so its text is what the client sees.
type badRequestError struct{ cause error }

func (e badRequestError) Error() string { return MsgBadRequest }
func (e badRequestError) Unwrap() error { return e.cause }

func init() {
	// The query parser is a package-level setting of grpc-gateway (the option only
	// assigns a global), so it is set once here rather than per mux.
	runtime.SetQueryParameterParser(queryParser{})(nil)
}

// queryParser is runtime.DefaultQueryParser with MsgBadRequest as the error text.
// A parameter with only empty values (`?kind=`, `?page=`) is treated as absent, like
// `?query=`: the default parser would reject an empty enum name or number.
type queryParser struct{}

func (queryParser) Parse(msg proto.Message, values url.Values, filter *utilities.DoubleArray) error {
	values = dropEmpty(values)
	if err := (&runtime.DefaultQueryParser{}).Parse(msg, values, filter); err != nil {
		return badRequestError{cause: err}
	}
	return nil
}

// dropEmpty returns values without the keys whose values are all empty strings.
func dropEmpty(values url.Values) url.Values {
	out := make(url.Values, len(values))
	for key, vals := range values {
		for _, v := range vals {
			if v != "" {
				out[key] = vals
				break
			}
		}
	}
	return out
}

// requestMarshaler decodes request bodies with MsgBadRequest as the error text;
// io.EOF (an empty body) passes through, the generated code accepts it.
//
// Unknown fields are discarded (DiscardUnknown), but protojson then also silently
// drops unknown enum names, turning e.g. "kind":"LISTING_KIND_NOPE" into
// UNSPECIFIED. SPEC §2.4 wants a parsing error for them, so enum names are checked
// against the request descriptor before decoding.
type requestMarshaler struct {
	*runtime.HTTPBodyMarshaler
}

func (m requestMarshaler) NewDecoder(r io.Reader) runtime.Decoder {
	return runtime.DecoderFunc(func(v any) error {
		var raw json.RawMessage
		if err := json.NewDecoder(r).Decode(&raw); err != nil {
			return wrapDecodeError(err)
		}
		return m.Unmarshal(raw, v)
	})
}

func (m requestMarshaler) Unmarshal(data []byte, v any) error {
	if pm, ok := v.(proto.Message); ok {
		if err := checkEnumNames(pm.ProtoReflect().Descriptor(), data); err != nil {
			return badRequestError{cause: err}
		}
	}
	return wrapDecodeError(m.HTTPBodyMarshaler.Unmarshal(data, v))
}

// checkEnumNames reports a string value of an enum field (also in lists and nested
// messages) that is not a name of that enum. Malformed JSON is left to protojson;
// null means "not set", as in protojson.
func checkEnumNames(md protoreflect.MessageDescriptor, data []byte) error {
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return nil
	}
	for key, val := range obj {
		fd := md.Fields().ByJSONName(key)
		if fd == nil {
			fd = md.Fields().ByTextName(key)
		}
		if fd == nil || fd.IsMap() {
			continue
		}
		values := []json.RawMessage{val}
		if fd.IsList() {
			values = nil
			if json.Unmarshal(val, &values) != nil {
				continue
			}
		}
		for _, v := range values {
			if string(v) == "null" {
				continue
			}
			switch {
			case fd.Enum() != nil:
				var name string
				if json.Unmarshal(v, &name) == nil && fd.Enum().Values().ByName(protoreflect.Name(name)) == nil {
					return fmt.Errorf("invalid value for enum field %s: %q", fd.Name(), name)
				}
			case fd.Message() != nil:
				if err := checkEnumNames(fd.Message(), v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func wrapDecodeError(err error) error {
	if err == nil || errors.Is(err, io.EOF) {
		return err
	}
	return badRequestError{cause: err}
}

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
		runtime.WithMarshalerOption(runtime.MIMEWildcard, requestMarshaler{&runtime.HTTPBodyMarshaler{
			Marshaler: &runtime.JSONPb{
				MarshalOptions:   protojson.MarshalOptions{EmitUnpopulated: true},
				UnmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
			},
		}}),
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
// «Внутренняя ошибка» (the details are logged by the gRPC side) and an ASCII
// WWW-Authenticate challenge on 401 (the default handler copies the message there).
func errorHandler(ctx context.Context, mux *runtime.ServeMux, m runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	st := status.Convert(err)
	if isServerError(st.Code()) {
		err = status.Error(st.Code(), MsgInternal)
	}
	if st.Code() == codes.Unauthenticated {
		w = challengeWriter{w}
	}
	runtime.DefaultHTTPErrorHandler(ctx, mux, m, w, r, err)
}

// challengeWriter sets WWW-Authenticate to "Bearer" right before the status line.
type challengeWriter struct{ http.ResponseWriter }

func (w challengeWriter) WriteHeader(code int) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.ResponseWriter.WriteHeader(code)
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
