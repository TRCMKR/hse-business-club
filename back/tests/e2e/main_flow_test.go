//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// Scenario 1 (SPEC §1): Анна публикует стартап → Борис находит его в ленте и
// откликается → Анна видит отклик и принимает → Борис видит «принят» в кабинете.
func TestMainScenario(t *testing.T) {
	e := newEnv(t)

	anna, annaUser := e.register("anna", "Анна Соколова")
	expectUser(t, annaUser)
	if annaUser["username"] != "anna" || annaUser["displayName"] != "Анна Соколова" || annaUser["avatarUrl"] != "" {
		t.Fatalf("anna = %v", annaUser)
	}
	boris, borisUser := e.register("boris", "Борис Климов")

	// Анна публикует стартап.
	created := e.createListing(anna, startup("FinTrack — учёт финансов"))
	expectListing(t, created)
	listingID := str(t, created, "id")
	if created["kind"] != "LISTING_KIND_STARTUP" || created["stage"] != "STARTUP_STAGE_MVP" ||
		created["status"] != "LISTING_STATUS_ACTIVE" || created["isMine"] != true || created["closedAt"] != nil ||
		created["myApplication"] != nil || created["compensation"] != "" {
		t.Fatalf("created listing = %v", created)
	}
	if st := obj(t, created, "stats"); num(st["total"]) != 0 || num(st["pending"]) != 0 {
		t.Fatalf("own new listing stats = %v, want zeros", st)
	}
	if obj(t, created, "author")["id"] != annaUser["id"] {
		t.Fatalf("author = %v", created["author"])
	}

	// Борис находит его в ленте: stats null для чужих, myApplication ещё null.
	feed := e.ok(http.MethodGet, "/v1/listings", boris, nil)
	expectKeys(t, "feed", feed, "items", "total", "page", "limit")
	if num(feed["total"]) != 1 || num(feed["page"]) != 1 || num(feed["limit"]) != 20 {
		t.Fatalf("feed = %v", feed)
	}
	it := items(t, feed)[0]
	expectListing(t, it)
	if it["id"] != listingID || it["isMine"] != false || it["stats"] != nil || it["myApplication"] != nil {
		t.Fatalf("feed item for boris = %v", it)
	}

	// Борис откликается.
	app := e.apply(boris, listingID, "  Готов взять бэкенд  ")
	expectApplication(t, app)
	appID := str(t, app, "id")
	if app["status"] != "APPLICATION_STATUS_PENDING" || app["message"] != "Готов взять бэкенд" || app["decidedAt"] != nil {
		t.Fatalf("application = %v", app)
	}
	if obj(t, app, "applicant")["id"] != borisUser["id"] || obj(t, app, "listing")["id"] != listingID {
		t.Fatalf("application refs = %v", app)
	}

	// Борис видит свой отклик в ленте и в деталях.
	it = items(t, e.ok(http.MethodGet, "/v1/listings", boris, nil))[0]
	if it["myApplication"] == nil || obj(t, it, "myApplication")["id"] != appID {
		t.Fatalf("feed myApplication = %v", it["myApplication"])
	}

	// Анна видит отклик: полученные, отклики на публикацию, статистика.
	received := items(t, e.ok(http.MethodGet, "/v1/me/applications/received", anna, nil))
	if len(received) != 1 || received[0]["id"] != appID || received[0]["status"] != "APPLICATION_STATUS_PENDING" {
		t.Fatalf("received = %v", received)
	}
	expectApplication(t, received[0])
	onListing := items(t, e.ok(http.MethodGet, "/v1/listings/"+listingID+"/applications", anna, nil))
	if len(onListing) != 1 || onListing[0]["id"] != appID {
		t.Fatalf("listing applications = %v", onListing)
	}
	mine := items(t, e.ok(http.MethodGet, "/v1/me/listings", anna, nil))
	if len(mine) != 1 {
		t.Fatalf("my listings = %v", mine)
	}
	if st := obj(t, mine[0], "stats"); num(st["pending"]) != 1 || num(st["total"]) != 1 {
		t.Fatalf("stats before accept = %v", st)
	}

	// Анна принимает.
	accepted := e.ok(http.MethodPost, "/v1/applications/"+appID+"/accept", anna, nil)
	expectApplication(t, accepted)
	if accepted["status"] != "APPLICATION_STATUS_ACCEPTED" || accepted["decidedAt"] == nil {
		t.Fatalf("accepted = %v", accepted)
	}
	if st := obj(t, items(t, e.ok(http.MethodGet, "/v1/me/listings", anna, nil))[0], "stats"); num(st["accepted"]) != 1 || num(st["pending"]) != 0 || num(st["total"]) != 1 {
		t.Fatalf("stats after accept = %v", st)
	}

	// Борис видит «принят» в кабинете и в деталях публикации.
	sent := items(t, e.ok(http.MethodGet, "/v1/me/applications/sent", boris, nil))
	if len(sent) != 1 || sent[0]["id"] != appID || sent[0]["status"] != "APPLICATION_STATUS_ACCEPTED" {
		t.Fatalf("sent = %v", sent)
	}
	sentAccepted := items(t, e.ok(http.MethodGet, "/v1/me/applications/sent?status=APPLICATION_STATUS_ACCEPTED", boris, nil))
	sentPending := items(t, e.ok(http.MethodGet, "/v1/me/applications/sent?status=APPLICATION_STATUS_PENDING", boris, nil))
	if len(sentAccepted) != 1 || len(sentPending) != 0 {
		t.Fatalf("sent filter: accepted %d, pending %d", len(sentAccepted), len(sentPending))
	}
	details := e.ok(http.MethodGet, "/v1/listings/"+listingID, boris, nil)
	expectListing(t, details)
	my := obj(t, details, "myApplication")
	if my["id"] != appID || my["status"] != "APPLICATION_STATUS_ACCEPTED" || details["stats"] != nil || details["isMine"] != false {
		t.Fatalf("details for boris = %v", details)
	}
}
