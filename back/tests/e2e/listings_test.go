//go:build e2e

package e2e

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Scenario 6 (SPEC §2.2–2.3): update by the author, closing (idempotent), a closed
// listing is hidden from the feed but readable by id, cannot be edited, and the
// "my listings" status filter.
func TestClosedListing(t *testing.T) {
	e := newEnv(t)
	anna, _ := e.register("anna", "Анна")
	boris, _ := e.register("boris", "Борис")

	created := e.createListing(anna, startup("FinTrack — учёт финансов"))
	id := str(t, created, "id")
	other := e.createListing(anna, vacancy("Ищем фронтендера"))
	otherID := str(t, other, "id")

	// Update by the author: fields change, kind stays, updatedAt moves.
	upd := e.ok(http.MethodPut, "/v1/listings/"+id, anna, J{
		"title": "FinTrack 2.0", "description": "Новое описание стартапа", "lookingFor": "Дизайнер",
		"tags": []string{"Fintech"}, "stage": "STARTUP_STAGE_LAUNCHED",
	})
	expectListing(t, upd)
	if upd["id"] != id || upd["kind"] != "LISTING_KIND_STARTUP" || upd["title"] != "FinTrack 2.0" ||
		upd["description"] != "Новое описание стартапа" || upd["lookingFor"] != "Дизайнер" ||
		upd["stage"] != "STARTUP_STAGE_LAUNCHED" || strings.Join(strs(upd["tags"]), ",") != "Fintech" ||
		upd["status"] != "LISTING_STATUS_ACTIVE" || upd["isMine"] != true {
		t.Fatalf("updated listing = %v", upd)
	}
	if upd["createdAt"] != created["createdAt"] {
		t.Errorf("createdAt changed: %v -> %v", created["createdAt"], upd["createdAt"])
	}
	if upd["updatedAt"] == created["updatedAt"] {
		t.Errorf("updatedAt did not change: %v", upd["updatedAt"])
	}
	if got := e.ok(http.MethodGet, "/v1/listings/"+id, boris, nil); got["title"] != "FinTrack 2.0" {
		t.Fatalf("update not visible to boris: %v", got)
	}

	// Close.
	closed := e.ok(http.MethodPost, "/v1/listings/"+id+"/close", anna, J{})
	expectListing(t, closed)
	if closed["status"] != "LISTING_STATUS_CLOSED" {
		t.Fatalf("closed status = %v", closed["status"])
	}
	closedAt := str(t, closed, "closedAt")
	expectTimestamp(t, closed, "closedAt")

	// Close again: 200, closedAt unchanged.
	again := e.ok(http.MethodPost, "/v1/listings/"+id+"/close", anna, J{})
	if again["status"] != "LISTING_STATUS_CLOSED" || again["closedAt"] != closedAt {
		t.Fatalf("second close = %v, want same closedAt %s", again, closedAt)
	}

	// Feed excludes it.
	feed := e.ok(http.MethodGet, "/v1/listings", boris, nil)
	if num(feed["total"]) != 1 {
		t.Fatalf("feed total = %v, want 1", feed["total"])
	}
	for _, it := range items(t, feed) {
		if it["id"] == id {
			t.Fatalf("closed listing is in the feed")
		}
	}
	if items(t, feed)[0]["id"] != otherID {
		t.Fatalf("feed = %v", feed)
	}

	// But it is readable by id for another user.
	got := e.ok(http.MethodGet, "/v1/listings/"+id, boris, nil)
	expectListing(t, got)
	if got["status"] != "LISTING_STATUS_CLOSED" || got["closedAt"] != closedAt || got["isMine"] != false {
		t.Fatalf("closed listing for boris = %v", got)
	}

	// Update of a closed listing by its author.
	e.fails(http.MethodPut, "/v1/listings/"+id, anna, J{
		"title": "Попытка правки", "description": "Описание стартапа для e2e-теста", "lookingFor": "Кто-то",
		"stage": "STARTUP_STAGE_MVP",
	}, http.StatusBadRequest, 9, "Публикация закрыта")

	// My listings with the status filter.
	all := items(t, e.ok(http.MethodGet, "/v1/me/listings", anna, nil))
	if len(all) != 2 {
		t.Fatalf("my listings = %d, want 2", len(all))
	}
	closedOnly := items(t, e.ok(http.MethodGet, "/v1/me/listings?status=LISTING_STATUS_CLOSED", anna, nil))
	if len(closedOnly) != 1 || closedOnly[0]["id"] != id {
		t.Fatalf("closed filter = %v", closedOnly)
	}
	activeOnly := items(t, e.ok(http.MethodGet, "/v1/me/listings?status=LISTING_STATUS_ACTIVE", anna, nil))
	if len(activeOnly) != 1 || activeOnly[0]["id"] != otherID {
		t.Fatalf("active filter = %v", activeOnly)
	}

	// Unknown and malformed ids.
	e.fails(http.MethodGet, "/v1/listings/"+uuid.NewString(), boris, nil, http.StatusNotFound, 5, "Публикация не найдена")
	e.fails(http.MethodGet, "/v1/listings/not-a-uuid", boris, nil, http.StatusNotFound, 5, "Публикация не найдена")
	e.fails(http.MethodPost, "/v1/listings/"+uuid.NewString()+"/close", anna, J{}, http.StatusNotFound, 5, "Публикация не найдена")
}

// Scenario 7 (SPEC §2.2, §2.4, §4.2): feed filters, search, order, pagination and
// validation of the feed query and of the create request.
func TestFeedFiltersAndValidation(t *testing.T) {
	e := newEnv(t)
	anna, _ := e.register("anna", "Анна")
	boris, _ := e.register("boris", "Борис")

	// Created in this order; the feed is newest first.
	s1 := e.createListing(anna, startup("Финансовый трекер"))
	v1 := e.createListing(boris, vacancy("Ищем Go-разработчика"))
	s2Req := startup("Сервис доставки")
	s2Req["description"] = "Платформа для КУРЬЕРОВ и ресторанов"
	s2 := e.createListing(boris, s2Req)
	v2 := e.createListing(anna, vacancy("Дизайнер интерфейсов"))
	s3 := e.createListing(anna, startup("EduBot"))
	created := []J{s1, v1, s2, v2, s3}
	ids := func(list []J) string {
		out := make([]string, len(list))
		for i, l := range list {
			out[i], _ = l["id"].(string)
		}
		return strings.Join(out, ",")
	}
	newestFirst := []J{s3, v2, s2, v1, s1}

	feed := e.ok(http.MethodGet, "/v1/listings", anna, nil)
	expectKeys(t, "feed", feed, "items", "total", "page", "limit")
	if num(feed["total"]) != len(created) || num(feed["page"]) != 1 || num(feed["limit"]) != 20 {
		t.Fatalf("feed meta = %v", feed)
	}
	if got := ids(items(t, feed)); got != ids(newestFirst) {
		t.Fatalf("feed order = %s, want %s", got, ids(newestFirst))
	}
	for _, it := range items(t, feed) {
		expectListing(t, it)
	}

	// Kind filter.
	st := e.ok(http.MethodGet, "/v1/listings?kind=LISTING_KIND_STARTUP", anna, nil)
	if num(st["total"]) != 3 || ids(items(t, st)) != ids([]J{s3, s2, s1}) {
		t.Fatalf("startups = %v", st)
	}
	vac := e.ok(http.MethodGet, "/v1/listings?kind=LISTING_KIND_VACANCY", anna, nil)
	if num(vac["total"]) != 2 || ids(items(t, vac)) != ids([]J{v2, v1}) {
		t.Fatalf("vacancies = %v", vac)
	}

	// Search: title and description, case-insensitive, Cyrillic.
	search := func(q string, extra string) J {
		return e.ok(http.MethodGet, "/v1/listings?query="+url.QueryEscape(q)+extra, anna, nil)
	}
	if r := search("ФИНАНСОВЫЙ", ""); ids(items(t, r)) != ids([]J{s1}) || num(r["total"]) != 1 {
		t.Fatalf("search title cyrillic = %v", r)
	}
	if r := search("курьеров", ""); ids(items(t, r)) != ids([]J{s2}) {
		t.Fatalf("search description = %v", r)
	}
	if r := search("go-РАЗРАБ", ""); ids(items(t, r)) != ids([]J{v1}) {
		t.Fatalf("search mixed case = %v", r)
	}
	if r := search("edubot", ""); ids(items(t, r)) != ids([]J{s3}) {
		t.Fatalf("search latin = %v", r)
	}
	// "Описание вакансии" is in both vacancies' descriptions; combined with kind.
	if r := search("описание ВАКАНСИИ", ""); ids(items(t, r)) != ids([]J{v2, v1}) {
		t.Fatalf("search vacancies = %v", r)
	}
	if r := search("стартапа", "&kind=LISTING_KIND_STARTUP"); ids(items(t, r)) != ids([]J{s3, s1}) {
		t.Fatalf("search + kind = %v", r)
	}
	if r := search("ничего такого нет", ""); len(items(t, r)) != 0 || num(r["total"]) != 0 {
		t.Fatalf("search nothing = %v", r)
	}
	// LIKE wildcards are literal.
	if r := search("%", ""); len(items(t, r)) != 0 {
		t.Fatalf("search %% = %v", r)
	}

	// Pagination.
	p2 := e.ok(http.MethodGet, "/v1/listings?page=2&limit=2", anna, nil)
	if num(p2["total"]) != 5 || num(p2["page"]) != 2 || num(p2["limit"]) != 2 || ids(items(t, p2)) != ids([]J{s2, v1}) {
		t.Fatalf("page 2 = %v", p2)
	}
	p3 := e.ok(http.MethodGet, "/v1/listings?page=3&limit=2", anna, nil)
	if ids(items(t, p3)) != ids([]J{s1}) {
		t.Fatalf("page 3 = %v", p3)
	}
	past := e.ok(http.MethodGet, "/v1/listings?page=10&limit=2", anna, nil)
	if len(items(t, past)) != 0 || num(past["total"]) != 5 || num(past["page"]) != 10 {
		t.Fatalf("past the end = %v", past)
	}
	// page=0 / limit=0 mean defaults.
	def := e.ok(http.MethodGet, "/v1/listings?page=0&limit=0", anna, nil)
	if num(def["page"]) != 1 || num(def["limit"]) != 20 || len(items(t, def)) != 5 {
		t.Fatalf("defaults = %v", def)
	}
	if r := e.ok(http.MethodGet, "/v1/listings?limit=50", anna, nil); num(r["limit"]) != 50 {
		t.Fatalf("limit 50 = %v", r)
	}

	// Feed query validation.
	bad := http.StatusBadRequest
	e.fails(http.MethodGet, "/v1/listings?limit=51", anna, nil, bad, 3, "Размер страницы: от 1 до 50")
	e.fails(http.MethodGet, "/v1/listings?limit=-1", anna, nil, bad, 3, "Размер страницы: от 1 до 50")
	e.fails(http.MethodGet, "/v1/listings?page=-1", anna, nil, bad, 3, "Страница: не меньше 1")
	e.fails(http.MethodGet, "/v1/listings?query="+strings.Repeat("я", 101), anna, nil, bad, 3, "Поиск: не больше 100 символов")
	if r := e.ok(http.MethodGet, "/v1/listings?query="+url.QueryEscape(strings.Repeat("я", 100)), anna, nil); num(r["total"]) != 0 {
		t.Fatalf("query of 100 = %v", r)
	}
	e.fails(http.MethodGet, "/v1/listings?kind=NOPE", anna, nil, bad, 3, "Некорректный формат запроса")
	e.fails(http.MethodGet, "/v1/listings?page=abc", anna, nil, bad, 3, "Некорректный формат запроса")

	// Create validation.
	create := func(base J, mut func(J), msg string) {
		t.Helper()
		mut(base)
		e.fails(http.MethodPost, "/v1/listings", anna, base, bad, 3, msg)
	}
	sv, vv := func() J { return startup("Валидный стартап") }, func() J { return vacancy("Валидная вакансия") }
	create(sv(), func(r J) { r["title"] = "ab" }, "Название: от 3 до 120 символов")
	create(sv(), func(r J) { r["title"] = "  ab  " }, "Название: от 3 до 120 символов")
	create(vv(), func(r J) { r["title"] = strings.Repeat("я", 121) }, "Название: от 3 до 120 символов")
	create(sv(), func(r J) { delete(r, "stage") }, "Стадия: укажите стадию")
	create(vv(), func(r J) { r["stage"] = "STARTUP_STAGE_IDEA" }, "Стадия: только для стартапа")
	create(sv(), func(r J) { r["compensation"] = "Зарплата" }, "Условия: только для вакансии")
	create(sv(), func(r J) { delete(r, "kind") }, "Вид публикации: выберите стартап или вакансию")
	create(vv(), func(r J) { r["tags"] = []string{"Go", " GO "} }, "Теги: без повторов")
	create(sv(), func(r J) { r["description"] = "коротко" }, "Описание: от 10 до 5000 символов")

	e.fails(http.MethodPost, "/v1/listings", anna, `{"title":`, bad, 3, "Некорректный формат запроса")
	e.fails(http.MethodPost, "/v1/listings", anna, `not json`, bad, 3, "Некорректный формат запроса")
	withBadStage := startup("Валидный стартап")
	withBadStage["stage"] = "MVP"
	e.fails(http.MethodPost, "/v1/listings", anna, withBadStage, bad, 3, "Некорректный формат запроса")
	withBadKind := startup("Валидный стартап")
	withBadKind["kind"] = "STARTUP"
	e.fails(http.MethodPost, "/v1/listings", anna, withBadKind, bad, 3, "Некорректный формат запроса")

	// Nothing invalid was created.
	if r := e.ok(http.MethodGet, "/v1/listings", anna, nil); num(r["total"]) != 5 {
		t.Fatalf("total after invalid creates = %v", r["total"])
	}
}
