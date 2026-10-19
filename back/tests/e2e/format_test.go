//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// Request parsing errors of the gateway are 400 «Некорректный формат запроса»
// (SPEC §2.4), not the English protojson text.
func TestRequestFormatErrors(t *testing.T) {
	e := newEnv(t)
	anna, _ := e.register("anna", "Анна")
	l := e.createListing(anna, startup("Публикация для формата"))
	const msg = "Некорректный формат запроса"

	cases := []struct {
		name, method, path, token string
		body                      any
	}{
		{"invalid json", http.MethodPost, "/v1/auth/register", "", `{"username":"x",`},
		{"array instead of object", http.MethodPost, "/v1/auth/login", "", `[]`},
		{"string instead of array", http.MethodPut, "/v1/me", anna, J{"displayName": "Анна", "skills": "Go"}},
		{"unknown enum name", http.MethodPut, "/v1/listings/" + str(t, l, "id"), anna, J{"title": "Новое", "stage": "STARTUP_STAGE_ROCKET"}},
		{"unknown enum in query", http.MethodGet, "/v1/me/listings?status=CLOSED", anna, nil},
		{"not a number in query", http.MethodGet, "/v1/listings?limit=ten", anna, nil},
	}
	parent := e.t
	defer func() { e.t = parent }()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e.t = t // report failures on the subtest
			e.fails(c.method, c.path, c.token, c.body, http.StatusBadRequest, 3, msg)
		})
	}
}
