package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const requestIDHeader = "X-Request-Id"

// trustedProxies may set X-Forwarded-For: loopback and private networks (the Nuxt
// server proxies API calls from the same host or docker network).
var trustedProxies = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7",
}

func (s *Server) newPublicRouter() (chi.Router, error) {
	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		exposeRequestID,
		middleware.ClientIPFromXFF(trustedProxies...),
		s.tel.HTTPMiddleware(),
		middleware.Recoverer,
		cors.Handler(cors.Options{
			AllowedOrigins:   s.cfg.CORSOrigins,
			AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
			AllowedHeaders:   []string{"Authorization", "Content-Type", requestIDHeader},
			ExposedHeaders:   []string{requestIDHeader},
			AllowCredentials: true,
			MaxAge:           300,
		}),
		middleware.RequestSize(MaxMessageSize),
	)
	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)

	// Register every gateway path as a chi route, so metrics and spans get the
	// route template as label; anything else still reaches the gateway (404/405 in
	// gateway format).
	routes, err := swaggerRoutes(s.swagger)
	if err != nil {
		return nil, err
	}
	gw := rejectBadQuery(s.gateway)
	for _, rt := range routes {
		r.Method(rt.method, rt.path, gw)
	}
	r.NotFound(gw.ServeHTTP)
	r.MethodNotAllowed(gw.ServeHTTP)
	return r, nil
}

// badQueryBody is the gateway-format error for an unparsable query string.
var badQueryBody, _ = json.Marshal(map[string]any{"code": 3, "message": MsgBadRequest, "details": []any{}})

// rejectBadQuery answers 400 «Некорректный формат запроса» for a query string with
// invalid escapes: the generated gateway code would return the English text of
// url.ParseQuery (the query parser is not reached).
func rejectBadQuery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := url.ParseQuery(r.URL.RawQuery); err != nil {
			writeJSON(w, http.StatusBadRequest, badQueryBody)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) newDebugRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Get("/healthz", s.handleHealthz)
	r.Get("/readyz", s.handleReadyz)
	r.Handle("/metrics", promhttp.HandlerFor(s.tel.Registry(), promhttp.HandlerOpts{Registry: s.tel.Registry()}))
	r.Mount("/debug", middleware.Profiler())

	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
	})
	r.Get("/docs/", serveSwaggerUI)
	r.Get("/docs/swagger.json", s.serveSwaggerJSON)

	// "Try it out" in Swagger UI calls the API on the same origin. The public router
	// gets a fresh chi route context: otherwise it appends its pattern to "/v1/*" and
	// metrics/spans are labelled "/v1/v1/ping" instead of "/v1/ping".
	r.Handle("/v1/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		s.public.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, nil)))
	}))
	return r
}

// exposeRequestID echoes the request id in the response headers.
func exposeRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set(requestIDHeader, id)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []byte(`{"status":"ok"}`))
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	checks := append([]namedCheck(nil), s.checks...)
	s.mu.RUnlock()

	results := make([]string, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
			defer cancel()
			if err := c.fn(ctx); err != nil {
				s.log.Warn("readiness check failed", zap.String("check", c.name), zap.Error(err))
				results[i] = err.Error()
				return
			}
			results[i] = "ok"
		})
	}
	wg.Wait()

	code := http.StatusOK
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, c := range checks {
		if results[i] != "ok" {
			code = http.StatusServiceUnavailable
		}
		if i > 0 {
			buf.WriteByte(',')
		}
		k, _ := json.Marshal(c.name)
		v, _ := json.Marshal(results[i])
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	writeJSON(w, code, buf.Bytes())
}

func writeJSON(w http.ResponseWriter, code int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

func (s *Server) serveSwaggerJSON(w http.ResponseWriter, _ *http.Request) {
	if s.swagger == nil {
		http.NotFound(w, nil)
		return
	}
	writeJSON(w, http.StatusOK, s.swagger)
}

const swaggerUIVersion = "5.17.14"

var swaggerUIPage = []byte(strings.ReplaceAll(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>hbc-back API</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@{v}/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@{v}/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/docs/swagger.json", dom_id: "#swagger-ui", persistAuthorization: true });
  </script>
</body>
</html>
`, "{v}", swaggerUIVersion))

func serveSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(swaggerUIPage)
}

type route struct {
	method, path string
}

// swaggerRoutes extracts (method, path template) pairs from an OpenAPI v2 document.
func swaggerRoutes(doc []byte) ([]route, error) {
	if len(doc) == 0 {
		return nil, nil
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(doc, &spec); err != nil {
		return nil, fmt.Errorf("server: parse swagger: %w", err)
	}
	var routes []route
	for path, ops := range spec.Paths {
		for method := range ops {
			m := strings.ToUpper(method)
			switch m {
			case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
				routes = append(routes, route{method: m, path: path})
			}
		}
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].path != routes[j].path {
			return routes[i].path < routes[j].path
		}
		return routes[i].method < routes[j].method
	})
	return routes, nil
}
