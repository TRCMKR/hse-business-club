//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// Scenario 9: SEED_DEMO_DATA=true on an empty database loads SPEC §6 through the
// context services; a second start does not duplicate it.
func TestSeed(t *testing.T) {
	e := newEnv(t, withSeed())
	if !strings.Contains(e.logs.String(), "demo data loaded") {
		t.Fatalf("no 'demo data loaded' in logs")
	}

	anna := e.login("anna")
	feed := e.ok(http.MethodGet, "/v1/listings", anna, nil)
	if num(feed["total"]) != 5 {
		t.Fatalf("feed total = %v, want 5", feed["total"])
	}
	byTitle := map[string]J{}
	for _, it := range items(t, feed) {
		expectListing(t, it)
		byTitle[str(t, it, "title")] = it
	}
	if _, ok := byTitle["Frontend-разработчик в Volunteer Hub"]; ok {
		t.Fatal("closed listing №6 is in the feed")
	}
	// Anna's own listings have stats, others do not.
	fin := byTitle["FinTrack — учёт личных финансов для студентов"]
	if fin == nil || fin["isMine"] != true {
		t.Fatalf("FinTrack in anna's feed = %v", fin)
	}
	if st := obj(t, fin, "stats"); num(st["pending"]) != 1 || num(st["accepted"]) != 1 || num(st["declined"]) != 0 || num(st["total"]) != 2 {
		t.Fatalf("FinTrack stats = %v, want pending 1, accepted 1, total 2", st)
	}
	// Anna applied to CampusEats: myApplication pending, stats null.
	ce := byTitle["CampusEats — доставка еды по кампусу Покровки"]
	if ce == nil || ce["stats"] != nil || ce["myApplication"] == nil ||
		obj(t, ce, "myApplication")["status"] != "APPLICATION_STATUS_PENDING" {
		t.Fatalf("CampusEats in anna's feed = %v", ce)
	}
	received := items(t, e.ok(http.MethodGet, "/v1/me/applications/received", anna, nil))
	if len(received) != 3 || received[0]["status"] != "APPLICATION_STATUS_PENDING" ||
		obj(t, received[0], "applicant")["username"] != "boris" {
		t.Fatalf("anna received = %v", received)
	}

	// Boris: pending on №1 and accepted on the closed №6.
	boris := e.login("boris")
	sent := statusByTitle(t, items(t, e.ok(http.MethodGet, "/v1/me/applications/sent", boris, nil)))
	if len(sent) != 2 || sent["FinTrack — учёт личных финансов для студентов"] != "APPLICATION_STATUS_PENDING" ||
		sent["Frontend-разработчик в Volunteer Hub"] != "APPLICATION_STATUS_ACCEPTED" {
		t.Fatalf("boris sent = %v", sent)
	}

	// Vera: accepted on №1 and withdrawn on №5.
	vera := e.login("vera")
	sent = statusByTitle(t, items(t, e.ok(http.MethodGet, "/v1/me/applications/sent", vera, nil)))
	if len(sent) != 2 || sent["FinTrack — учёт личных финансов для студентов"] != "APPLICATION_STATUS_ACCEPTED" ||
		sent["Volunteer Hub — платформа волонтёрских проектов ВШЭ"] != "APPLICATION_STATUS_WITHDRAWN" {
		t.Fatalf("vera sent = %v", sent)
	}

	// Dmitry: №5 (only a withdrawn application → stats.total = 0, SPEC §4.2) and
	// the closed №6 with the accepted application of Boris.
	dmitry := e.login("dmitry")
	mine := items(t, e.ok(http.MethodGet, "/v1/me/listings", dmitry, nil))
	if len(mine) != 2 {
		t.Fatalf("dmitry listings = %d, want 2", len(mine))
	}
	for _, l := range mine {
		expectListing(t, l)
		st := obj(t, l, "stats")
		switch str(t, l, "title") {
		case "Volunteer Hub — платформа волонтёрских проектов ВШЭ":
			if num(st["total"]) != 0 || num(st["pending"]) != 0 {
				t.Errorf("№5 stats = %v, want all zero (withdrawn is not counted)", st)
			}
		case "Frontend-разработчик в Volunteer Hub":
			if l["status"] != "LISTING_STATUS_CLOSED" || l["closedAt"] == nil || num(st["accepted"]) != 1 || num(st["total"]) != 1 {
				t.Errorf("№6 = %v, want closed with 1 accepted", l)
			}
		default:
			t.Errorf("unexpected dmitry listing %v", l["title"])
		}
	}
	// Dmitry's application to №2 was declined by Anna.
	sent = statusByTitle(t, items(t, e.ok(http.MethodGet, "/v1/me/applications/sent", dmitry, nil)))
	if sent["Backend-разработчик в FinTrack"] != "APPLICATION_STATUS_DECLINED" {
		t.Fatalf("dmitry sent = %v", sent)
	}

	// Profiles are filled, photos are absent.
	me := e.ok(http.MethodGet, "/v1/me", anna, nil)
	expectUser(t, me)
	if me["headline"] != "Фаундер FinTrack, ex-продакт в финтехе" || me["telegram"] != "anna_sokolova" ||
		strings.Join(strs(me["skills"]), ",") != "Продукт,Финтех,Питчинг" || me["avatarUrl"] != "" {
		t.Fatalf("anna profile = %v", me)
	}

	// Vera may apply again to №5 after withdrawing.
	vh := byTitle["Volunteer Hub — платформа волонтёрских проектов ВШЭ"]
	e.apply(vera, str(t, vh, "id"), "Появилось время, могу помочь с дизайном.")

	// Second start with the seed on a non-empty database: skipped, nothing doubled.
	e2 := startApp(t, withSeed())
	if !strings.Contains(e2.logs.String(), "demo data skipped") {
		t.Fatalf("no 'demo data skipped' in logs of the second start")
	}
	if got := num(e2.ok(http.MethodGet, "/v1/listings", e2.login("anna"), nil)["total"]); got != 5 {
		t.Fatalf("feed total after second start = %d, want 5", got)
	}
	var users int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM users.users").Scan(&users); err != nil || users != 4 {
		t.Fatalf("users = %d (%v), want 4", users, err)
	}
}

func statusByTitle(t *testing.T, apps []J) map[string]any {
	t.Helper()
	out := map[string]any{}
	for _, a := range apps {
		expectApplication(t, a)
		out[str(t, obj(t, a, "listing"), "title")] = a["status"]
	}
	return out
}
