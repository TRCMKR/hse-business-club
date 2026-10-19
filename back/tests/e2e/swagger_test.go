//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// Every path and method of hbc.swagger.json (what the frontend client is generated
// from) is served by the gateway, and only ping/register/login/avatars are public.
func TestSwaggerMatchesRoutes(t *testing.T) {
	e := newEnv(t)
	tok, _ := e.register("anna", "Анна")

	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(hbcv1.SwaggerJSON(), &doc); err != nil {
		t.Fatal(err)
	}
	public := map[string]bool{
		"GET /v1/ping": true, "POST /v1/auth/register": true, "POST /v1/auth/login": true,
		"GET /v1/avatars/{file}": true,
	}
	param := regexp.MustCompile(`\{[^}]+\}`)

	var n int
	for p, ops := range doc.Paths {
		for method := range ops {
			method = strings.ToUpper(method)
			route := method + " " + p
			concrete := param.ReplaceAllStringFunc(p, func(string) string {
				if strings.Contains(p, "{file}") {
					return uuid.NewString() + ".jpg"
				}
				return uuid.NewString()
			})
			n++

			for _, token := range []string{"", tok} {
				var body any
				if method == http.MethodPost || method == http.MethodPut {
					body = J{}
				}
				code, out := e.do(method, concrete, token, body)
				if msg := out["message"]; code == http.StatusMethodNotAllowed || code == http.StatusNotImplemented ||
					msg == "Не найдено" || msg == "Метод не поддерживается" {
					t.Errorf("%s (%s): not routed: %d %v", route, concrete, code, out)
				}
				if token == "" && !public[route] && code != http.StatusUnauthorized {
					t.Errorf("%s without token = %d %v, want 401", route, code, out)
				}
				// Login with an empty body is a legitimate 401 «Неверный логин или пароль».
				if token == "" && public[route] && code == http.StatusUnauthorized && out["message"] == "Требуется вход" {
					t.Errorf("%s is public, got 401", route)
				}
			}
		}
	}
	if n != 22 {
		t.Errorf("swagger has %d operations, want 22 (SPEC §4.1)", n)
	}
}
