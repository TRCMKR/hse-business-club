package service_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob/memory"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
	"github.com/trcmkr/hse-business-club/back/internal/users/service"
)

const (
	testUser     = "anna"
	testPassword = "password1"
)

type env struct {
	svc    users.Service
	repo   *fakeRepo
	hasher *fakeHasher
	store  *memory.Store
}

func newEnv() *env {
	e := &env{repo: newFakeRepo(), hasher: &fakeHasher{}, store: memory.New()}
	e.svc = service.New(e.repo, e.hasher, e.store)
	return e
}

func wantErr(t *testing.T, err error, kind apperr.Kind, msg string) {
	t.Helper()
	var e *apperr.Error
	if !errors.As(err, &e) {
		t.Fatalf("want *apperr.Error %v %q, got %v", kind, msg, err)
	}
	if e.Kind != kind || (msg != "" && e.Msg != msg) {
		t.Fatalf("want %v %q, got %v %q", kind, msg, e.Kind, e.Msg)
	}
}

func (e *env) register(t *testing.T, username string) *users.User {
	t.Helper()
	u, err := e.svc.Register(context.Background(), users.RegisterParams{Username: username, Password: testPassword, DisplayName: "Имя"})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func pngImage(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRegisterAndAuthenticate(t *testing.T) {
	e := newEnv()
	ctx := context.Background()

	u, err := e.svc.Register(ctx, users.RegisterParams{Username: " Anna_K ", Password: testPassword, DisplayName: " Анна "})
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "anna_k" || u.DisplayName != "Анна" {
		t.Fatalf("not normalized: %+v", u)
	}
	if h := e.repo.hashes[u.ID]; h != "hash:password1" {
		t.Fatalf("stored hash %q", h)
	}

	got, err := e.svc.Authenticate(ctx, "ANNA_K", testPassword)
	if err != nil || got.ID != u.ID {
		t.Fatalf("Authenticate = %+v, %v", got, err)
	}

	_, err = e.svc.Register(ctx, users.RegisterParams{Username: "anna_k", Password: "password2", DisplayName: "Другая"})
	wantErr(t, err, apperr.KindConflict, "Логин уже занят")

	_, err = e.svc.Register(ctx, users.RegisterParams{Username: "a", Password: testPassword, DisplayName: "A"})
	wantErr(t, err, apperr.KindValidation, domain.MsgUsername)
	_, err = e.svc.Register(ctx, users.RegisterParams{Username: "bob", Password: "short", DisplayName: "B"})
	wantErr(t, err, apperr.KindValidation, domain.MsgPassword)
}

func TestAuthenticateFailures(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	e.register(t, testUser)

	for _, tc := range []struct{ name, username, password string }{
		{"wrong password", testUser, "password2"},
		{"unknown user", "nobody", testPassword},
		{"malformed login", "не логин", testPassword},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := e.hasher.count()
			_, err := e.svc.Authenticate(ctx, tc.username, tc.password)
			wantErr(t, err, apperr.KindUnauthenticated, service.MsgInvalidCredentials)
			if e.hasher.count() != before+1 {
				t.Fatalf("expected exactly one hash comparison, got %d", e.hasher.count()-before)
			}
		})
	}

	t.Run("storage failure is not masked", func(t *testing.T) {
		e.repo.err = apperr.Wrap(apperr.KindUnknown, "Внутренняя ошибка", errors.New("db down"))
		defer func() { e.repo.err = nil }()
		_, err := e.svc.Authenticate(ctx, testUser, testPassword)
		wantErr(t, err, apperr.KindUnknown, "")
	})
}

// The production hasher really is bcrypt and the dummy hash path works with it.
func TestBcryptHasher(t *testing.T) {
	repo := newFakeRepo()
	svc := service.New(repo, service.Bcrypt, memory.New())
	ctx := context.Background()
	u, err := svc.Register(ctx, users.RegisterParams{Username: testUser, Password: testPassword, DisplayName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if h := repo.hashes[u.ID]; !strings.HasPrefix(h, "$2a$10$") {
		t.Fatalf("not a bcrypt cost-10 hash: %q", h)
	}
	if _, err := svc.Authenticate(ctx, testUser, testPassword); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Authenticate(ctx, "ghost", testPassword)
	wantErr(t, err, apperr.KindUnauthenticated, service.MsgInvalidCredentials)
}

func TestGetAndGetMany(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	a, b := e.register(t, "alice"), e.register(t, "bob")

	got, err := e.svc.Get(ctx, a.ID)
	if err != nil || got.Username != "alice" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	_, err = e.svc.Get(ctx, uuid.New())
	wantErr(t, err, apperr.KindNotFound, "")

	m, err := e.svc.GetMany(ctx, []uuid.UUID{a.ID, uuid.New(), b.ID, a.ID})
	if err != nil || len(m) != 2 || m[a.ID] == nil || m[b.ID] == nil {
		t.Fatalf("GetMany = %v, %v", m, err)
	}
	m, err = e.svc.GetMany(ctx, nil)
	if err != nil || m == nil || len(m) != 0 {
		t.Fatalf("GetMany(nil) = %v, %v", m, err)
	}
}

func TestUpdateProfile(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	u := e.register(t, testUser)

	got, err := e.svc.UpdateProfile(ctx, u.ID, users.UpdateProfileParams{
		DisplayName: " Анна ", Headline: "Продакт", Skills: []string{" Go ", "", "SQL"}, Telegram: "@anna",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Анна" || got.Telegram != testUser || len(got.Skills) != 2 || got.Skills[0] != "Go" {
		t.Fatalf("not normalized: %+v", got)
	}

	_, err = e.svc.UpdateProfile(ctx, u.ID, users.UpdateProfileParams{DisplayName: "A", Skills: []string{"Go", "go"}})
	wantErr(t, err, apperr.KindValidation, "Навыки: без повторов")
	_, err = e.svc.UpdateProfile(ctx, u.ID, users.UpdateProfileParams{DisplayName: "A", Headline: strings.Repeat("x", 121)})
	wantErr(t, err, apperr.KindValidation, "Кратко о себе: не больше 120 символов")
	_, err = e.svc.UpdateProfile(ctx, uuid.New(), users.UpdateProfileParams{DisplayName: "A"})
	wantErr(t, err, apperr.KindNotFound, "")
}

func TestAvatarLifecycle(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	u := e.register(t, testUser)

	// set: one object in the store, key in the profile
	u1, err := e.svc.SetAvatar(ctx, u.ID, pngImage(t, 300, 200, color.RGBA{R: 200, A: 255}))
	if err != nil {
		t.Fatal(err)
	}
	if u1.AvatarKey == "" || !strings.HasSuffix(u1.AvatarKey, ".jpg") || e.store.Len() != 1 || !e.store.Has(u1.AvatarKey) {
		t.Fatalf("after set: key %q, store %d", u1.AvatarKey, e.store.Len())
	}
	av, err := e.svc.OpenAvatar(ctx, u1.AvatarKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(av.Data))
	if err != nil || format != "jpeg" || cfg.Width != 512 || cfg.Height != 512 || av.ContentType != "image/jpeg" {
		t.Fatalf("stored avatar: %v %s %dx%d %s", err, format, cfg.Width, cfg.Height, av.ContentType)
	}

	// second set: new key, old object deleted
	u2, err := e.svc.SetAvatar(ctx, u.ID, pngImage(t, 100, 100, color.RGBA{B: 200, A: 255}))
	if err != nil {
		t.Fatal(err)
	}
	if u2.AvatarKey == u1.AvatarKey || e.store.Len() != 1 || e.store.Has(u1.AvatarKey) || !e.store.Has(u2.AvatarKey) {
		t.Fatalf("after second set: %q → %q, store %d", u1.AvatarKey, u2.AvatarKey, e.store.Len())
	}
	_, err = e.svc.OpenAvatar(ctx, u1.AvatarKey)
	wantErr(t, err, apperr.KindNotFound, service.MsgAvatarNotFound)

	// invalid photo: validation, nothing changes
	_, err = e.svc.SetAvatar(ctx, u.ID, []byte("not an image"))
	wantErr(t, err, apperr.KindValidation, domain.MsgAvatarFormat)
	if cur, _ := e.svc.Get(ctx, u.ID); cur.AvatarKey != u2.AvatarKey || e.store.Len() != 1 {
		t.Fatal("invalid photo changed the profile or the store")
	}

	// remove, twice: idempotent
	for range 2 {
		got, err := e.svc.RemoveAvatar(ctx, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.AvatarKey != "" || e.store.Len() != 0 {
			t.Fatalf("after remove: key %q, store %d", got.AvatarKey, e.store.Len())
		}
	}

	// unknown user
	_, err = e.svc.SetAvatar(ctx, uuid.New(), pngImage(t, 64, 64, color.Black))
	wantErr(t, err, apperr.KindNotFound, "")
	_, err = e.svc.RemoveAvatar(ctx, uuid.New())
	wantErr(t, err, apperr.KindNotFound, "")
	if e.store.Len() != 0 {
		t.Fatalf("store has %d objects for an unknown user", e.store.Len())
	}
}

// If saving the key fails after the upload, the uploaded object is removed.
func TestSetAvatarCleansUpOnRepoFailure(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	u := e.register(t, testUser)
	failing := &failingSetKey{fakeRepo: e.repo}
	svc := service.New(failing, e.hasher, e.store)
	_, err := svc.SetAvatar(ctx, u.ID, pngImage(t, 64, 64, color.White))
	wantErr(t, err, apperr.KindUnknown, "")
	if e.store.Len() != 0 {
		t.Fatalf("orphan object left: %d", e.store.Len())
	}
}

type failingSetKey struct{ *fakeRepo }

func (f *failingSetKey) SetAvatarKey(context.Context, uuid.UUID, string) (*users.User, error) {
	return nil, apperr.Wrap(apperr.KindUnknown, "Внутренняя ошибка", errors.New("db down"))
}

func TestOpenAvatar(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	foreign := uuid.NewString() + ".jpg"
	_ = e.store.Put(ctx, "secret.txt", "text/plain", strings.NewReader("x"), 1)

	for _, key := range []string{
		"", foreign, "../" + foreign, "secret.txt", strings.ToUpper(uuid.NewString()) + ".jpg",
		uuid.NewString() + ".png", uuid.NewString() + ".jpg/x",
	} {
		_, err := e.svc.OpenAvatar(ctx, key)
		wantErr(t, err, apperr.KindNotFound, service.MsgAvatarNotFound)
	}
}

func TestOpenAvatarStoreFailure(t *testing.T) {
	svc := service.New(newFakeRepo(), &fakeHasher{}, brokenStore{})
	_, err := svc.OpenAvatar(context.Background(), uuid.NewString()+".jpg")
	wantErr(t, err, apperr.KindUnknown, "")
}

type brokenStore struct{ blob.Store }

func (brokenStore) Get(context.Context, string) (*blob.Object, error) {
	return nil, errors.New("s3 down")
}

// Errors leaving the service are *apperr.Error and never mention the password hash.
func TestNoHashInErrors(t *testing.T) {
	e := newEnv()
	e.register(t, testUser)
	_, err := e.svc.Authenticate(context.Background(), testUser, "wrong-password")
	if strings.Contains(err.Error(), "hash") {
		t.Fatalf("error leaks hash: %v", err)
	}
}

// If the hasher fails in New, misses are still compared against a real bcrypt hash.
func TestDummyHashFallback(t *testing.T) {
	h := &brokenHasher{}
	svc := service.New(newFakeRepo(), h, memory.New())
	_, err := svc.Authenticate(context.Background(), "ghost", testPassword)
	wantErr(t, err, apperr.KindUnauthenticated, service.MsgInvalidCredentials)
	if len(h.checked) != 1 || !strings.HasPrefix(h.checked[0], "$2a$10$") || len(h.checked[0]) != 60 {
		t.Fatalf("miss compared against %q, want a bcrypt cost-10 hash", h.checked)
	}
}

// The miss hash is computed in New, not on the first miss.
func TestDummyHashComputedInNew(t *testing.T) {
	h := &countingHasher{}
	svc := service.New(newFakeRepo(), h, memory.New())
	if n := h.hashes; n != 1 {
		t.Fatalf("Hash called %d times in New, want 1", n)
	}
	_, _ = svc.Authenticate(context.Background(), "ghost", testPassword)
	_, _ = svc.Authenticate(context.Background(), "ghost2", testPassword)
	if n := h.hashes; n != 1 {
		t.Fatalf("Hash called %d times after misses, want 1", n)
	}
}

type brokenHasher struct{ checked []string }

func (*brokenHasher) Hash(string) (string, error) { return "", errors.New("rng failure") }
func (b *brokenHasher) Check(hash, _ string) bool {
	b.checked = append(b.checked, hash)
	return false
}

type countingHasher struct {
	fakeHasher
	hashes int
}

func (c *countingHasher) Hash(p string) (string, error) {
	c.hashes++
	return c.fakeHasher.Hash(p)
}
