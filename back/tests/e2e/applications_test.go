//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	statusPending   = "APPLICATION_STATUS_PENDING"
	statusAccepted  = "APPLICATION_STATUS_ACCEPTED"
	statusDeclined  = "APPLICATION_STATUS_DECLINED"
	statusWithdrawn = "APPLICATION_STATUS_WITHDRAWN"

	msgForbidden      = "Недостаточно прав"
	msgAppNotFound    = "Отклик не найден"
	msgAlreadyDecided = "Решение по отклику уже принято"
	msgUnauth         = "Требуется вход"
)

func appPath(id, action string) string { return "/v1/applications/" + id + "/" + action }

// expectStatus checks the status of an application and the presence of decidedAt.
func expectStatus(t *testing.T, a J, status string, decided bool) {
	t.Helper()
	expectApplication(t, a)
	if a["status"] != status {
		t.Fatalf("application status = %v, want %s (%v)", a["status"], status, a)
	}
	if (a["decidedAt"] != nil) != decided {
		t.Fatalf("application decidedAt = %v, want set=%v", a["decidedAt"], decided)
	}
}

// Scenario 2 (SPEC §4.2): repeat application, withdraw (idempotent), re-apply
// with a new id; withdrawn applications are excluded from the author's stats.
func TestApplicationWithdrawAndReapply(t *testing.T) {
	e := newEnv(t)
	anna, _ := e.register("anna", "Анна Соколова")
	boris, _ := e.register("boris", "Борис Климов")
	vera, _ := e.register("vera", "Вера Ким")

	listingID := str(t, e.createListing(anna, vacancy("Frontend-разработчик")), "id")
	first := e.apply(boris, listingID, "Хочу к вам")
	firstID := str(t, first, "id")

	// Repeat application while the first one is pending.
	e.fails(http.MethodPost, "/v1/listings/"+listingID+"/applications", boris, J{"message": "Ещё раз"},
		http.StatusConflict, 6, "Вы уже откликались на эту публикацию")

	// Withdraw, then withdraw again (idempotent).
	w := e.ok(http.MethodPost, appPath(firstID, "withdraw"), boris, nil)
	expectStatus(t, w, statusWithdrawn, false)
	if w["id"] != firstID {
		t.Fatalf("withdrawn id = %v, want %s", w["id"], firstID)
	}
	w2 := e.ok(http.MethodPost, appPath(firstID, "withdraw"), boris, nil)
	expectStatus(t, w2, statusWithdrawn, false)
	if w2["updatedAt"] != w["updatedAt"] {
		t.Errorf("repeat withdraw changed updatedAt: %v -> %v", w["updatedAt"], w2["updatedAt"])
	}

	// Apply again: a new pending application.
	second := e.apply(boris, listingID, "Передумал, снова хочу")
	expectStatus(t, second, statusPending, false)
	secondID := str(t, second, "id")
	if secondID == firstID {
		t.Fatalf("re-apply returned the withdrawn id %s", firstID)
	}

	// Listing details show the new application.
	details := e.ok(http.MethodGet, "/v1/listings/"+listingID, boris, nil)
	expectListing(t, details)
	if my := obj(t, details, "myApplication"); my["id"] != secondID || my["status"] != statusPending {
		t.Fatalf("myApplication = %v, want %s pending", my, secondID)
	}

	// A declined application from another user counts in total; withdrawn does not.
	veraApp := str(t, e.apply(vera, listingID, "Я тоже"), "id")
	expectStatus(t, e.ok(http.MethodPost, appPath(veraApp, "decline"), anna, nil), statusDeclined, true)

	mine := items(t, e.ok(http.MethodGet, "/v1/me/listings", anna, nil))
	if len(mine) != 1 {
		t.Fatalf("my listings = %v", mine)
	}
	st := obj(t, mine[0], "stats")
	if num(st["pending"]) != 1 || num(st["accepted"]) != 0 || num(st["declined"]) != 1 || num(st["total"]) != 2 {
		t.Fatalf("stats = %v, want pending 1, accepted 0, declined 1, total 2", st)
	}
	// The own listing details carry the same stats.
	if st := obj(t, e.ok(http.MethodGet, "/v1/listings/"+listingID, anna, nil), "stats"); num(st["total"]) != 2 {
		t.Fatalf("details stats = %v, want total 2", st)
	}
}

// Scenario 3 (SPEC §2.3, §4.2): applications that are rejected by the rules.
func TestApplicationApplyRejected(t *testing.T) {
	e := newEnv(t)
	anna, _ := e.register("anna", "Анна Соколова")
	boris, _ := e.register("boris", "Борис Климов")
	listingID := str(t, e.createListing(anna, startup("FinTrack")), "id")
	applyPath := "/v1/listings/" + listingID + "/applications"

	e.fails(http.MethodPost, applyPath, anna, J{"message": "Сам себе"},
		http.StatusBadRequest, 9, "Нельзя откликнуться на свою публикацию")

	e.fails(http.MethodPost, applyPath, boris, J{"message": ""},
		http.StatusBadRequest, 3, "Сообщение: от 1 до 2000 символов")
	e.fails(http.MethodPost, applyPath, boris, J{"message": "   "},
		http.StatusBadRequest, 3, "Сообщение: от 1 до 2000 символов")

	e.fails(http.MethodPost, "/v1/listings/9b2c1f5e-3a4d-4e6f-8a7b-1c2d3e4f5a6b/applications", boris,
		J{"message": "Привет"}, http.StatusNotFound, 5, "Публикация не найдена")

	closed := e.ok(http.MethodPost, "/v1/listings/"+listingID+"/close", anna, nil)
	if closed["status"] != "LISTING_STATUS_CLOSED" {
		t.Fatalf("closed listing = %v", closed)
	}
	e.fails(http.MethodPost, applyPath, boris, J{"message": "Поздно"},
		http.StatusBadRequest, 9, "Публикация закрыта")
}

// Scenario 4 (SPEC §2.3, DECISIONS 01:45): the other party gets 403, an
// outsider gets 404; authentication errors are 401.
func TestApplicationPermissions(t *testing.T) {
	e := newEnv(t)
	anna, _ := e.register("anna", "Анна Соколова")
	boris, borisUser := e.register("boris", "Борис Климов")
	gleb, _ := e.register("gleb", "Глеб Орлов")

	listingID := str(t, e.createListing(anna, startup("FinTrack")), "id")
	appID := str(t, e.apply(boris, listingID, "Готов"), "id")

	// Outsider: application is not visible.
	e.fails(http.MethodPost, appPath(appID, "accept"), gleb, nil, http.StatusNotFound, 5, msgAppNotFound)
	e.fails(http.MethodPost, appPath(appID, "decline"), gleb, nil, http.StatusNotFound, 5, msgAppNotFound)
	e.fails(http.MethodPost, appPath(appID, "withdraw"), gleb, nil, http.StatusNotFound, 5, msgAppNotFound)
	// Other party: forbidden.
	e.fails(http.MethodPost, appPath(appID, "accept"), boris, nil, http.StatusForbidden, 7, msgForbidden)
	e.fails(http.MethodPost, appPath(appID, "decline"), boris, nil, http.StatusForbidden, 7, msgForbidden)
	e.fails(http.MethodPost, appPath(appID, "withdraw"), anna, nil, http.StatusForbidden, 7, msgForbidden)
	// Non-existent application.
	e.fails(http.MethodPost, appPath("9b2c1f5e-3a4d-4e6f-8a7b-1c2d3e4f5a6b", "accept"), anna, nil,
		http.StatusNotFound, 5, msgAppNotFound)

	// Nothing changed.
	if a := items(t, e.ok(http.MethodGet, "/v1/me/applications/sent", boris, nil)); len(a) != 1 || a[0]["status"] != statusPending {
		t.Fatalf("sent after forbidden actions = %v", a)
	}

	// Listing-level permissions.
	e.fails(http.MethodGet, "/v1/listings/"+listingID+"/applications", boris, nil, http.StatusForbidden, 7, msgForbidden)
	e.fails(http.MethodGet, "/v1/listings/"+listingID+"/applications", gleb, nil, http.StatusForbidden, 7, msgForbidden)
	upd := startup("Чужой заголовок")
	delete(upd, "kind")
	e.fails(http.MethodPut, "/v1/listings/"+listingID, boris, upd, http.StatusForbidden, 7, msgForbidden)
	e.fails(http.MethodPost, "/v1/listings/"+listingID+"/close", boris, nil, http.StatusForbidden, 7, msgForbidden)
	if l := e.ok(http.MethodGet, "/v1/listings/"+listingID, anna, nil); l["title"] != "FinTrack" || l["status"] != "LISTING_STATUS_ACTIVE" {
		t.Fatalf("listing changed by a stranger: %v", l)
	}
	// The author's update with the same body works (body is valid).
	if l := e.ok(http.MethodPut, "/v1/listings/"+listingID, anna, upd); l["title"] != "Чужой заголовок" {
		t.Fatalf("author update = %v", l)
	}

	// No token.
	resp, body := e.raw(http.MethodGet, "/v1/me", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /v1/me without token = %d %s", resp.StatusCode, body)
	}
	if h := resp.Header.Get("WWW-Authenticate"); h != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", h)
	}
	e.fails(http.MethodGet, "/v1/me", "", nil, http.StatusUnauthorized, 16, msgUnauth)

	// Garbage token.
	e.fails(http.MethodGet, "/v1/me", "not-a-jwt", nil, http.StatusUnauthorized, 16, msgUnauth)

	// Well-formed JWT of a real user signed with another secret.
	now := time.Now()
	forged, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   str(t, borisUser, "id"),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}).SignedString([]byte("another-secret-another-secret-12"))
	if err != nil {
		t.Fatal(err)
	}
	e.fails(http.MethodGet, "/v1/me", forged, nil, http.StatusUnauthorized, 16, msgUnauth)
	// The genuine token still works.
	if me := e.ok(http.MethodGet, "/v1/me", boris, nil); me["id"] != borisUser["id"] {
		t.Fatalf("GET /v1/me = %v", me)
	}
}

// Scenario 5 (SPEC §4.2): decisions are final; repeated accept is idempotent.
func TestApplicationDecisions(t *testing.T) {
	e := newEnv(t)
	anna, _ := e.register("anna", "Анна Соколова")
	boris, _ := e.register("boris", "Борис Климов")
	vera, _ := e.register("vera", "Вера Ким")
	listingID := str(t, e.createListing(anna, startup("FinTrack")), "id")

	borisApp := str(t, e.apply(boris, listingID, "Готов"), "id")
	acc := e.ok(http.MethodPost, appPath(borisApp, "accept"), anna, nil)
	expectStatus(t, acc, statusAccepted, true)
	acc2 := e.ok(http.MethodPost, appPath(borisApp, "accept"), anna, nil)
	expectStatus(t, acc2, statusAccepted, true)
	if acc2["decidedAt"] != acc["decidedAt"] || acc2["updatedAt"] != acc["updatedAt"] {
		t.Errorf("repeat accept changed timestamps: %v/%v -> %v/%v",
			acc["decidedAt"], acc["updatedAt"], acc2["decidedAt"], acc2["updatedAt"])
	}
	e.fails(http.MethodPost, appPath(borisApp, "decline"), anna, nil, http.StatusBadRequest, 9, msgAlreadyDecided)
	e.fails(http.MethodPost, appPath(borisApp, "withdraw"), boris, nil, http.StatusBadRequest, 9, msgAlreadyDecided)

	veraApp := str(t, e.apply(vera, listingID, "Тоже готова"), "id")
	dec := e.ok(http.MethodPost, appPath(veraApp, "decline"), anna, nil)
	expectStatus(t, dec, statusDeclined, true)
	dec2 := e.ok(http.MethodPost, appPath(veraApp, "decline"), anna, nil)
	expectStatus(t, dec2, statusDeclined, true)
	if dec2["decidedAt"] != dec["decidedAt"] {
		t.Errorf("repeat decline changed decidedAt: %v -> %v", dec["decidedAt"], dec2["decidedAt"])
	}
	e.fails(http.MethodPost, appPath(veraApp, "accept"), anna, nil, http.StatusBadRequest, 9, msgAlreadyDecided)
	e.fails(http.MethodPost, appPath(veraApp, "withdraw"), vera, nil, http.StatusBadRequest, 9, msgAlreadyDecided)

	// Final states are what the applicants see.
	if s := items(t, e.ok(http.MethodGet, "/v1/me/applications/sent", boris, nil)); len(s) != 1 || s[0]["status"] != statusAccepted {
		t.Fatalf("boris sent = %v", s)
	}
	if s := items(t, e.ok(http.MethodGet, "/v1/me/applications/sent", vera, nil)); len(s) != 1 || s[0]["status"] != statusDeclined {
		t.Fatalf("vera sent = %v", s)
	}
}

// SPEC §2.3: received applications are sorted pending first, then created_at
// DESC; the status filter works, an unknown one is a format error.
func TestReceivedApplicationsOrderAndFilter(t *testing.T) {
	e := newEnv(t)
	anna, _ := e.register("anna", "Анна Соколова")
	boris, _ := e.register("boris", "Борис Климов")
	vera, _ := e.register("vera", "Вера Ким")
	gleb, _ := e.register("gleb", "Глеб Орлов")
	l1 := str(t, e.createListing(anna, startup("FinTrack")), "id")
	l2 := str(t, e.createListing(anna, vacancy("Дизайнер")), "id")

	borisApp := str(t, e.apply(boris, l1, "Первый"), "id")
	veraApp := str(t, e.apply(vera, l2, "Второй"), "id")
	glebApp := str(t, e.apply(gleb, l1, "Третий"), "id")
	e.ok(http.MethodPost, appPath(veraApp, "accept"), anna, nil)

	ids := func(list []J) []string {
		out := make([]string, len(list))
		for i, a := range list {
			out[i], _ = a["id"].(string)
		}
		return out
	}
	expectIDs := func(what string, got []J, want ...string) {
		t.Helper()
		g := ids(got)
		if len(g) != len(want) {
			t.Fatalf("%s = %v, want %v", what, g, want)
		}
		for i := range want {
			if g[i] != want[i] {
				t.Fatalf("%s = %v, want %v", what, g, want)
			}
		}
	}

	received := items(t, e.ok(http.MethodGet, "/v1/me/applications/received", anna, nil))
	for _, a := range received {
		expectApplication(t, a)
	}
	expectIDs("received", received, glebApp, borisApp, veraApp)

	pending := items(t, e.ok(http.MethodGet, "/v1/me/applications/received?status="+statusPending, anna, nil))
	expectIDs("received pending", pending, glebApp, borisApp)
	accepted := items(t, e.ok(http.MethodGet, "/v1/me/applications/received?status="+statusAccepted, anna, nil))
	expectIDs("received accepted", accepted, veraApp)

	// Per-listing list uses the same order.
	expectIDs("listing applications", items(t, e.ok(http.MethodGet, "/v1/listings/"+l1+"/applications", anna, nil)),
		glebApp, borisApp)

	// Applicants do not receive anything.
	expectIDs("boris received", items(t, e.ok(http.MethodGet, "/v1/me/applications/received", boris, nil)))

	e.fails(http.MethodGet, "/v1/me/applications/received?status=NOPE", anna, nil,
		http.StatusBadRequest, 3, "Некорректный формат запроса")
	e.fails(http.MethodGet, "/v1/me/applications/sent?status=NOPE", boris, nil,
		http.StatusBadRequest, 3, "Некорректный формат запроса")
}
