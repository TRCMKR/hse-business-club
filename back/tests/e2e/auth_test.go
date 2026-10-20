//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// noSecrets fails if a raw response body leaks a password or a bcrypt hash. Values
// of the issued tokens are skipped: random strings may contain anything.
func noSecrets(t *testing.T, what string, body []byte) {
	t.Helper()
	var out J
	if json.Unmarshal(body, &out) == nil {
		delete(out, "accessToken")
		delete(out, "refreshToken")
		body, _ = json.Marshal(out)
	}
	s := strings.ToLower(string(body))
	for _, bad := range []string{"password", "$2a$", "$2b$", "hash"} {
		if strings.Contains(s, bad) {
			t.Errorf("%s leaks %q: %s", what, bad, body)
		}
	}
}

// Scenario 8 (SPEC §2.1, §2.4, §4.1): registration and login.
func TestRegisterAndLogin(t *testing.T) {
	e := newEnv(t)
	bad := http.StatusBadRequest
	reg := func(username, pwd string) J {
		return J{"username": username, "password": pwd, "displayName": "Анна"}
	}

	e.fails(http.MethodPost, "/v1/auth/register", "", reg("anna", "1234567"), bad, 3, "Пароль: от 8 до 72 символов")
	e.fails(http.MethodPost, "/v1/auth/register", "", reg("anna", strings.Repeat("a", 73)), bad, 3, "Пароль: от 8 до 72 символов")
	const badLogin = "Логин: от 3 до 32 символов, латиница, цифры и _"
	e.fails(http.MethodPost, "/v1/auth/register", "", reg("Bad Name!", password), bad, 3, badLogin)
	e.fails(http.MethodPost, "/v1/auth/register", "", reg("ab", password), bad, 3, badLogin)
	e.fails(http.MethodPost, "/v1/auth/register", "", reg("анна", password), bad, 3, badLogin)
	e.fails(http.MethodPost, "/v1/auth/register", "", reg(strings.Repeat("a", 33), password), bad, 3, badLogin)
	e.fails(http.MethodPost, "/v1/auth/register", "", `{"username":`, bad, 3, "Некорректный формат запроса")

	// Username is trimmed and lower-cased; the response has no secrets.
	resp, body := e.raw(http.MethodPost, "/v1/auth/register", "", reg("  Anna_1 ", password))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register Anna_1 = %d %s", resp.StatusCode, body)
	}
	noSecrets(t, "register response", body)
	_, out := e.do(http.MethodPost, "/v1/auth/login", "", J{"username": "anna_1", "password": password})
	expectPair(t, "auth response", out)
	u := obj(t, out, "user")
	expectUser(t, u)
	if u["username"] != "anna_1" || u["displayName"] != "Анна" || u["headline"] != "" || u["avatarUrl"] != "" ||
		len(strs(u["skills"])) != 0 {
		t.Fatalf("registered user = %v", u)
	}
	annaID := str(t, u, "id")

	// Duplicates, including another case.
	e.fails(http.MethodPost, "/v1/auth/register", "", reg("anna_1", password), http.StatusConflict, 6, "Логин уже занят")
	e.fails(http.MethodPost, "/v1/auth/register", "", reg("ANNA_1", password), http.StatusConflict, 6, "Логин уже занят")

	// Login.
	unauth := http.StatusUnauthorized
	const wrong = "Неверный логин или пароль"
	e.fails(http.MethodPost, "/v1/auth/login", "", J{"username": "anna_1", "password": "wrong-password"}, unauth, 16, wrong)
	e.fails(http.MethodPost, "/v1/auth/login", "", J{"username": "nobody", "password": password}, unauth, 16, wrong)
	e.fails(http.MethodPost, "/v1/auth/login", "", J{"username": "Bad Name!", "password": password}, unauth, 16, wrong)

	// Login with another case works too; the token works for /v1/me.
	tok := e.login("ANNA_1")
	resp, body = e.raw(http.MethodGet, "/v1/me", tok, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/me = %d %s", resp.StatusCode, body)
	}
	noSecrets(t, "GET /v1/me", body)
	me := e.ok(http.MethodGet, "/v1/me", tok, nil)
	expectUser(t, me)
	if me["id"] != annaID || me["username"] != "anna_1" {
		t.Fatalf("me = %v", me)
	}

	// No or broken token.
	e.fails(http.MethodGet, "/v1/me", "", nil, unauth, 16, "Требуется вход")
	e.fails(http.MethodGet, "/v1/me", "garbage", nil, unauth, 16, "Требуется вход")
}

// Scenario 8, profile part (SPEC §2.1, §2.4): update own profile, view another
// member's profile.
func TestProfile(t *testing.T) {
	e := newEnv(t)
	anna, annaUser := e.register("anna", "Анна")
	boris, _ := e.register("boris", "Борис")
	annaID := str(t, annaUser, "id")

	upd := e.ok(http.MethodPut, "/v1/me", anna, J{
		"displayName": "  Анна Соколова ", "headline": "Go-разработчица",
		"about": "Пишу бэкенды", "skills": []string{"Go", " PostgreSQL ", ""}, "telegram": " @anna_s ",
	})
	expectUser(t, upd)
	if upd["id"] != annaID || upd["username"] != "anna" || upd["displayName"] != "Анна Соколова" ||
		upd["headline"] != "Go-разработчица" || upd["about"] != "Пишу бэкенды" ||
		strings.Join(strs(upd["skills"]), ",") != "Go,PostgreSQL" || upd["telegram"] != "anna_s" {
		t.Fatalf("updated profile = %v", upd)
	}
	if upd["createdAt"] != annaUser["createdAt"] {
		t.Errorf("createdAt changed: %v -> %v", annaUser["createdAt"], upd["createdAt"])
	}

	resp, body := e.raw(http.MethodGet, "/v1/users/"+annaID, boris, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET user = %d %s", resp.StatusCode, body)
	}
	noSecrets(t, "GET /v1/users/{id}", body)
	got := e.ok(http.MethodGet, "/v1/users/"+annaID, boris, nil)
	expectUser(t, got)
	for _, k := range userKeys {
		if k == "skills" {
			continue
		}
		if got[k] != upd[k] {
			t.Errorf("profile for boris: %s = %v, want %v", k, got[k], upd[k])
		}
	}
	if strings.Join(strs(got["skills"]), ",") != "Go,PostgreSQL" {
		t.Errorf("profile for boris: skills = %v", got["skills"])
	}
	if me := e.ok(http.MethodGet, "/v1/me", anna, nil); me["telegram"] != "anna_s" || me["displayName"] != "Анна Соколова" {
		t.Fatalf("me after update = %v", me)
	}

	e.fails(http.MethodGet, "/v1/users/"+uuid.NewString(), boris, nil, http.StatusNotFound, 5, "Участник не найден")
	e.fails(http.MethodGet, "/v1/users/not-a-uuid", boris, nil, http.StatusNotFound, 5, "Участник не найден")
	e.fails(http.MethodGet, "/v1/users/"+annaID, "", nil, http.StatusUnauthorized, 16, "Требуется вход")

	bad := http.StatusBadRequest
	e.fails(http.MethodPut, "/v1/me", anna, J{"displayName": "Анна", "skills": []string{"Go", "go"}}, bad, 3, "Навыки: без повторов")
	e.fails(http.MethodPut, "/v1/me", anna, J{"displayName": "   "}, bad, 3, "Имя: от 1 до 80 символов")
	e.fails(http.MethodPut, "/v1/me", anna, J{"displayName": "Анна", "headline": strings.Repeat("я", 121)}, bad, 3, "Кратко о себе: не больше 120 символов")

	// Failed updates changed nothing.
	if me := e.ok(http.MethodGet, "/v1/me", anna, nil); strings.Join(strs(me["skills"]), ",") != "Go,PostgreSQL" {
		t.Fatalf("me after failed updates = %v", me)
	}
}
