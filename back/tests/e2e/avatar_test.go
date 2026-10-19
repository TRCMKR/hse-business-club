//go:build e2e

package e2e

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func avatarBody(data []byte) J {
	return J{"data": base64.StdEncoding.EncodeToString(data)}
}

// Scenario 10: profile photo upload, serving, replacement and removal.
func TestAvatar(t *testing.T) {
	e := newEnv(t)
	anna, _ := e.register("anna", "Анна")
	boris, _ := e.register("boris", "Борис")

	me := e.ok(http.MethodPost, "/v1/me/avatar", anna, avatarBody(testPNG(t, 300, 200)))
	expectUser(t, me)
	url1 := str(t, me, "avatarUrl")
	if !strings.HasPrefix(url1, "/v1/avatars/") || !strings.HasSuffix(url1, ".jpg") {
		t.Fatalf("avatarUrl = %q", url1)
	}

	// Served without a token as an immutable 512×512 JPEG.
	resp, data := e.raw(http.MethodGet, url1, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d %s", url1, resp.StatusCode, data)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", cc)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode avatar: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("avatar is %dx%d, want 512x512", b.Dx(), b.Dy())
	}

	// The author's photo is visible in the feed and in applications.
	l := e.createListing(anna, startup("Стартап с фото"))
	feed := items(t, e.ok(http.MethodGet, "/v1/listings", boris, nil))
	if obj(t, feed[0], "author")["avatarUrl"] != url1 {
		t.Fatalf("feed author avatarUrl = %v, want %s", obj(t, feed[0], "author")["avatarUrl"], url1)
	}
	borisMe := e.ok(http.MethodPost, "/v1/me/avatar", boris, avatarBody(testPNG(t, 64, 64)))
	e.apply(boris, str(t, l, "id"), "Привет")
	received := items(t, e.ok(http.MethodGet, "/v1/me/applications/received", anna, nil))
	if obj(t, received[0], "applicant")["avatarUrl"] != borisMe["avatarUrl"] || borisMe["avatarUrl"] == "" {
		t.Fatalf("applicant avatarUrl = %v, want %v", obj(t, received[0], "applicant")["avatarUrl"], borisMe["avatarUrl"])
	}

	// Re-upload: a new name, the old file is gone.
	url2 := str(t, e.ok(http.MethodPost, "/v1/me/avatar", anna, avatarBody(testPNG(t, 200, 300))), "avatarUrl")
	if url2 == url1 || url2 == "" {
		t.Fatalf("re-upload avatarUrl = %q (old %q)", url2, url1)
	}
	e.fails(http.MethodGet, url1, "", nil, http.StatusNotFound, 5, "Фото не найдено")
	if resp, _ := e.raw(http.MethodGet, url2, "", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET new avatar = %d", resp.StatusCode)
	}
	e.fails(http.MethodGet, "/v1/avatars/nope.jpg", "", nil, http.StatusNotFound, 5, "Фото не найдено")

	// Not an image.
	e.fails(http.MethodPost, "/v1/me/avatar", anna, avatarBody([]byte("definitely not an image")),
		http.StatusBadRequest, 3, "Фото: поддерживаются JPEG, PNG и WebP")
	e.fails(http.MethodPost, "/v1/me/avatar", anna, avatarBody(testPNG(t, 32, 32)),
		http.StatusBadRequest, 3, "Фото: минимум 64×64")
	// The failed uploads did not touch the current photo.
	if got := e.ok(http.MethodGet, "/v1/me", anna, nil)["avatarUrl"]; got != url2 {
		t.Fatalf("avatarUrl after failed uploads = %v, want %s", got, url2)
	}
	// Uploading requires a token.
	e.fails(http.MethodPost, "/v1/me/avatar", "", avatarBody(testPNG(t, 64, 64)), http.StatusUnauthorized, 16, "Требуется вход")

	// Delete (idempotent).
	if got := e.ok(http.MethodDelete, "/v1/me/avatar", anna, nil)["avatarUrl"]; got != "" {
		t.Fatalf("avatarUrl after delete = %v", got)
	}
	if got := e.ok(http.MethodDelete, "/v1/me/avatar", anna, nil)["avatarUrl"]; got != "" {
		t.Fatalf("avatarUrl after second delete = %v", got)
	}
	e.fails(http.MethodGet, url2, "", nil, http.StatusNotFound, 5, "Фото не найдено")
	if n := e.blob.Len(); n != 1 { // only boris's photo is left
		t.Fatalf("blob store has %d objects, want 1", n)
	}
}
