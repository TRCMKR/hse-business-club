package service_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob"
	blobmocks "github.com/trcmkr/hse-business-club/back/internal/platform/blob/mocks"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
	"github.com/trcmkr/hse-business-club/back/internal/users/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/users/service"
	"github.com/trcmkr/hse-business-club/back/internal/users/service/mocks"
)

var (
	_ service.Repository        = (*postgres.Repository)(nil)
	_ service.RefreshRepository = (*postgres.RefreshRepository)(nil)
)

const (
	testUser       = "anna"
	testPassword   = "password1"
	testRefreshTTL = 24 * time.Hour
	dummyHash      = "dummy-hash" // what the mocked hasher returns in New
	storedHash     = "stored-hash"
	msgInternal    = "Внутренняя ошибка"
)

var avatarKeyRe = regexp.MustCompile(`^[0-9a-f-]{36}\.jpg$`)

type env struct {
	svc    users.Service
	repo   *mocks.MockRepository
	tokens *mocks.MockRefreshRepository
	hasher *mocks.MockHasher
	store  *blobmocks.MockStore
	now    time.Time // the service clock
}

// newEnv builds the service on mocks; New hashes a random password once for misses.
func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		repo: mocks.NewMockRepository(t), tokens: mocks.NewMockRefreshRepository(t),
		hasher: mocks.NewMockHasher(t), store: blobmocks.NewMockStore(t),
		now: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
	}
	e.hasher.EXPECT().Hash(mock.Anything).Return(dummyHash, nil).Once()
	e.svc = service.New(e.repo, e.tokens, e.hasher, e.store,
		service.WithRefreshTTL(testRefreshTTL), service.WithClock(func() time.Time { return e.now }))
	return e
}

func wantErr(t *testing.T, err error, kind apperr.Kind, msg string) {
	t.Helper()
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	require.Equal(t, kind, ae.Kind, "err = %v", err)
	if msg != "" {
		require.Equal(t, msg, ae.Msg)
	}
}

func dbDown() error { return apperr.Wrap(apperr.KindUnknown, msgInternal, errors.New("db down")) }

func userNotFound() error { return apperr.New(apperr.KindNotFound, postgres.MsgUserNotFound) }

func user(username, avatarKey string) *users.User {
	return &users.User{ID: uuid.New(), Username: username, DisplayName: "Имя", Skills: []string{}, AvatarKey: avatarKey}
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
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("normalized and hashed", func(t *testing.T) {
		e := newEnv(t)
		created := user("anna_k", "")
		e.hasher.EXPECT().Hash(testPassword).Return("hash:password1", nil).Once()
		e.repo.EXPECT().Create(mock.Anything, "anna_k", "hash:password1", "Анна").Return(created, nil).Once()

		u, err := e.svc.Register(ctx, users.RegisterParams{Username: " Anna_K ", Password: testPassword, DisplayName: " Анна "})
		require.NoError(t, err)
		assert.Same(t, created, u)
	})
	t.Run("username taken", func(t *testing.T) {
		e := newEnv(t)
		e.hasher.EXPECT().Hash(mock.Anything).Return("hash", nil).Once()
		e.repo.EXPECT().Create(mock.Anything, "anna_k", "hash", "Другая").
			Return(nil, apperr.New(apperr.KindConflict, postgres.MsgUsernameTaken)).Once()
		_, err := e.svc.Register(ctx, users.RegisterParams{Username: "anna_k", Password: "password2", DisplayName: "Другая"})
		wantErr(t, err, apperr.KindConflict, "Логин уже занят")
	})
	t.Run("invalid input reaches neither hasher nor repo", func(t *testing.T) {
		e := newEnv(t)
		_, err := e.svc.Register(ctx, users.RegisterParams{Username: "a", Password: testPassword, DisplayName: "A"})
		wantErr(t, err, apperr.KindValidation, domain.MsgUsername)
		_, err = e.svc.Register(ctx, users.RegisterParams{Username: "bob", Password: "short", DisplayName: "B"})
		wantErr(t, err, apperr.KindValidation, domain.MsgPassword)
	})
	t.Run("hasher failure", func(t *testing.T) {
		e := newEnv(t)
		e.hasher.EXPECT().Hash(testPassword).Return("", errors.New("rng failure")).Once()
		_, err := e.svc.Register(ctx, users.RegisterParams{Username: testUser, Password: testPassword, DisplayName: "A"})
		wantErr(t, err, apperr.KindUnknown, msgInternal)
	})
}

func TestAuthenticate(t *testing.T) {
	ctx := context.Background()

	t.Run("normalized login, password checked against the stored hash", func(t *testing.T) {
		e := newEnv(t)
		u := user("anna_k", "")
		e.repo.EXPECT().GetByUsername(mock.Anything, "anna_k").Return(u, storedHash, nil).Once()
		e.hasher.EXPECT().Check(storedHash, testPassword).Return(true).Once()
		got, err := e.svc.Authenticate(ctx, " ANNA_K ", testPassword)
		require.NoError(t, err)
		assert.Same(t, u, got)
	})

	// Every failure is the same error and costs exactly one hash comparison
	// (the mocks fail on a second Check or on a missing one).
	t.Run("wrong password", func(t *testing.T) {
		e := newEnv(t)
		e.repo.EXPECT().GetByUsername(mock.Anything, testUser).Return(user(testUser, ""), storedHash, nil).Once()
		e.hasher.EXPECT().Check(storedHash, "password2").Return(false).Once()
		_, err := e.svc.Authenticate(ctx, testUser, "password2")
		wantErr(t, err, apperr.KindUnauthenticated, service.MsgInvalidCredentials)
		assert.NotContains(t, err.Error(), "hash", "error leaks the hash")
	})
	t.Run("unknown user", func(t *testing.T) {
		e := newEnv(t)
		e.repo.EXPECT().GetByUsername(mock.Anything, "nobody").Return(nil, "", userNotFound()).Once()
		e.hasher.EXPECT().Check(dummyHash, testPassword).Return(false).Once()
		_, err := e.svc.Authenticate(ctx, "nobody", testPassword)
		wantErr(t, err, apperr.KindUnauthenticated, service.MsgInvalidCredentials)
	})
	for name, username := range map[string]string{"malformed login": "не логин", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t) // no repo lookup
			e.hasher.EXPECT().Check(dummyHash, mock.Anything).Return(false).Once()
			_, err := e.svc.Authenticate(ctx, username, "")
			wantErr(t, err, apperr.KindUnauthenticated, service.MsgInvalidCredentials)
		})
	}
	t.Run("storage failure is not masked", func(t *testing.T) {
		e := newEnv(t)
		e.repo.EXPECT().GetByUsername(mock.Anything, testUser).Return(nil, "", dbDown()).Once()
		_, err := e.svc.Authenticate(ctx, testUser, testPassword)
		wantErr(t, err, apperr.KindUnknown, "")
	})
}

// The production hasher really is bcrypt and the dummy hash path works with it.
func TestBcryptHasher(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockRepository(t)
	svc := service.New(repo, mocks.NewMockRefreshRepository(t), service.Bcrypt, blobmocks.NewMockStore(t))
	u := user(testUser, "")

	var hash string
	repo.EXPECT().Create(mock.Anything, testUser, mock.Anything, "A").
		Run(func(_ context.Context, _, h, _ string) { hash = h }).Return(u, nil).Once()
	_, err := svc.Register(ctx, users.RegisterParams{Username: testUser, Password: testPassword, DisplayName: "A"})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(hash, "$2a$10$"), "not a bcrypt cost-10 hash: %q", hash)

	repo.EXPECT().GetByUsername(mock.Anything, testUser).Return(u, hash, nil).Once()
	got, err := svc.Authenticate(ctx, testUser, testPassword)
	require.NoError(t, err)
	assert.Same(t, u, got)

	repo.EXPECT().GetByUsername(mock.Anything, testUser).Return(u, hash, nil).Once()
	_, err = svc.Authenticate(ctx, testUser, "password2")
	wantErr(t, err, apperr.KindUnauthenticated, service.MsgInvalidCredentials)

	repo.EXPECT().GetByUsername(mock.Anything, "ghost").Return(nil, "", userNotFound()).Once()
	_, err = svc.Authenticate(ctx, "ghost", testPassword)
	wantErr(t, err, apperr.KindUnauthenticated, service.MsgInvalidCredentials)
}

// If the hasher fails in New, misses are still compared against a real bcrypt hash.
func TestDummyHashFallback(t *testing.T) {
	isBcrypt := mock.MatchedBy(func(h string) bool { return strings.HasPrefix(h, "$2a$10$") && len(h) == 60 })
	for name, hashErr := range map[string]error{"error": errors.New("rng failure"), "empty hash": nil} {
		t.Run(name, func(t *testing.T) {
			repo, hasher := mocks.NewMockRepository(t), mocks.NewMockHasher(t)
			hasher.EXPECT().Hash(mock.Anything).Return("", hashErr).Once()
			svc := service.New(repo, mocks.NewMockRefreshRepository(t), hasher, blobmocks.NewMockStore(t))

			repo.EXPECT().GetByUsername(mock.Anything, "ghost").Return(nil, "", userNotFound()).Once()
			hasher.EXPECT().Check(isBcrypt, testPassword).Return(false).Once()
			_, err := svc.Authenticate(context.Background(), "ghost", testPassword)
			wantErr(t, err, apperr.KindUnauthenticated, service.MsgInvalidCredentials)
		})
	}
}

// The miss hash is computed in New, not on the first miss: newEnv expects exactly one
// Hash call, and the misses only compare.
func TestDummyHashComputedInNew(t *testing.T) {
	e := newEnv(t)
	for _, name := range []string{"ghost", "ghost2"} {
		e.repo.EXPECT().GetByUsername(mock.Anything, name).Return(nil, "", userNotFound()).Once()
	}
	e.hasher.EXPECT().Check(dummyHash, testPassword).Return(false).Twice()
	_, _ = e.svc.Authenticate(context.Background(), "ghost", testPassword)
	_, _ = e.svc.Authenticate(context.Background(), "ghost2", testPassword)
}

func TestGetAndGetMany(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a, b := user("alice", ""), user("bob", "")

	e.repo.EXPECT().GetByID(mock.Anything, a.ID).Return(a, nil).Once()
	got, err := e.svc.Get(ctx, a.ID)
	require.NoError(t, err)
	assert.Same(t, a, got)

	missing := uuid.New()
	e.repo.EXPECT().GetByID(mock.Anything, missing).Return(nil, userNotFound()).Once()
	_, err = e.svc.Get(ctx, missing)
	wantErr(t, err, apperr.KindNotFound, "Участник не найден")

	// duplicates are dropped, the order of first occurrences is kept
	found := map[uuid.UUID]*users.User{a.ID: a, b.ID: b}
	e.repo.EXPECT().GetMany(mock.Anything, []uuid.UUID{a.ID, missing, b.ID}).Return(found, nil).Once()
	m, err := e.svc.GetMany(ctx, []uuid.UUID{a.ID, missing, b.ID, a.ID})
	require.NoError(t, err)
	assert.Equal(t, found, m)

	m, err = e.svc.GetMany(ctx, nil) // no repo call
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.Empty(t, m)
}

func TestUpdateProfile(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	u := user(testUser, "")

	want := users.UpdateProfileParams{DisplayName: "Анна", Headline: "Продакт", Skills: []string{"Go", "SQL"}, Telegram: testUser}
	e.repo.EXPECT().Update(mock.Anything, u.ID, want).Return(u, nil).Once()
	got, err := e.svc.UpdateProfile(ctx, u.ID, users.UpdateProfileParams{
		DisplayName: " Анна ", Headline: "Продакт", Skills: []string{" Go ", "", "SQL"}, Telegram: "@anna",
	})
	require.NoError(t, err)
	assert.Same(t, u, got)

	// invalid input does not reach the repo
	_, err = e.svc.UpdateProfile(ctx, u.ID, users.UpdateProfileParams{DisplayName: "A", Skills: []string{"Go", "go"}})
	wantErr(t, err, apperr.KindValidation, "Навыки: без повторов")
	_, err = e.svc.UpdateProfile(ctx, u.ID, users.UpdateProfileParams{DisplayName: "A", Headline: strings.Repeat("x", 121)})
	wantErr(t, err, apperr.KindValidation, "Кратко о себе: не больше 120 символов")

	missing := uuid.New()
	e.repo.EXPECT().Update(mock.Anything, missing, mock.Anything).Return(nil, userNotFound()).Once()
	_, err = e.svc.UpdateProfile(ctx, missing, users.UpdateProfileParams{DisplayName: "A"})
	wantErr(t, err, apperr.KindNotFound, "")
}

// putAvatar expects one Put of a normalized avatar and records its key and bytes.
type putAvatar struct {
	key  string
	data []byte
}

func (e *env) expectPut(t *testing.T, err error) *putAvatar {
	t.Helper()
	p := &putAvatar{}
	e.store.EXPECT().Put(mock.Anything, mock.Anything, "image/jpeg", mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, key, _ string, r io.Reader, size int64) error {
			data, rerr := io.ReadAll(r)
			require.NoError(t, rerr)
			require.Equal(t, int64(len(data)), size)
			p.key, p.data = key, data
			return err
		}).Once()
	return p
}

// expectSetKey expects the key of the last Put to be saved.
func (e *env) expectSetKey(t *testing.T, u *users.User, p *putAvatar, err error) {
	t.Helper()
	e.repo.EXPECT().SetAvatarKey(mock.Anything, u.ID, mock.Anything).RunAndReturn(
		func(_ context.Context, _ uuid.UUID, key string) (*users.User, error) {
			assert.Equal(t, p.key, key, "the uploaded key is saved")
			if err != nil {
				return nil, err
			}
			c := *u
			c.AvatarKey = key
			return &c, nil
		}).Once()
}

func TestSetAvatar(t *testing.T) {
	ctx := context.Background()

	t.Run("first photo: normalized 512×512 JPEG under a new key", func(t *testing.T) {
		e := newEnv(t)
		u := user(testUser, "")
		e.repo.EXPECT().GetByID(mock.Anything, u.ID).Return(u, nil).Once()
		put := e.expectPut(t, nil)
		e.expectSetKey(t, u, put, nil) // no Delete: there was no previous photo

		got, err := e.svc.SetAvatar(ctx, u.ID, pngImage(t, 300, 200, color.RGBA{R: 200, A: 255}))
		require.NoError(t, err)
		assert.Regexp(t, avatarKeyRe, got.AvatarKey)
		assert.Equal(t, put.key, got.AvatarKey)
		cfg, format, err := image.DecodeConfig(bytes.NewReader(put.data))
		require.NoError(t, err)
		assert.Equal(t, "jpeg", format)
		assert.Equal(t, 512, cfg.Width)
		assert.Equal(t, 512, cfg.Height)
	})
	t.Run("replacing: new key, old object deleted", func(t *testing.T) {
		e := newEnv(t)
		old := uuid.NewString() + ".jpg"
		u := user(testUser, old)
		e.repo.EXPECT().GetByID(mock.Anything, u.ID).Return(u, nil).Once()
		put := e.expectPut(t, nil)
		e.expectSetKey(t, u, put, nil)
		e.store.EXPECT().Delete(mock.Anything, old).Return(nil).Once()

		got, err := e.svc.SetAvatar(ctx, u.ID, pngImage(t, 100, 100, color.RGBA{B: 200, A: 255}))
		require.NoError(t, err)
		assert.NotEqual(t, old, got.AvatarKey)
		assert.Equal(t, put.key, got.AvatarKey)
	})
	t.Run("failed delete of the old object is only logged", func(t *testing.T) {
		e := newEnv(t)
		old := uuid.NewString() + ".jpg"
		u := user(testUser, old)
		e.repo.EXPECT().GetByID(mock.Anything, u.ID).Return(u, nil).Once()
		put := e.expectPut(t, nil)
		e.expectSetKey(t, u, put, nil)
		e.store.EXPECT().Delete(mock.Anything, old).Return(errors.New("s3 down")).Once()

		got, err := e.svc.SetAvatar(ctx, u.ID, pngImage(t, 64, 64, color.White))
		require.NoError(t, err)
		assert.Equal(t, put.key, got.AvatarKey)
	})
	t.Run("invalid photo: nothing is touched", func(t *testing.T) {
		e := newEnv(t)
		_, err := e.svc.SetAvatar(ctx, uuid.New(), []byte("not an image"))
		wantErr(t, err, apperr.KindValidation, domain.MsgAvatarFormat)
		_, err = e.svc.SetAvatar(ctx, uuid.New(), pngImage(t, 63, 64, color.White))
		wantErr(t, err, apperr.KindValidation, domain.MsgAvatarMinSide)
	})
	t.Run("unknown user: nothing is uploaded", func(t *testing.T) {
		e := newEnv(t)
		id := uuid.New()
		e.repo.EXPECT().GetByID(mock.Anything, id).Return(nil, userNotFound()).Once()
		_, err := e.svc.SetAvatar(ctx, id, pngImage(t, 64, 64, color.Black))
		wantErr(t, err, apperr.KindNotFound, "")
	})
	t.Run("upload failure: the key is not saved", func(t *testing.T) {
		e := newEnv(t)
		u := user(testUser, "")
		e.repo.EXPECT().GetByID(mock.Anything, u.ID).Return(u, nil).Once()
		e.expectPut(t, errors.New("s3 down"))
		_, err := e.svc.SetAvatar(ctx, u.ID, pngImage(t, 64, 64, color.White))
		wantErr(t, err, apperr.KindUnknown, msgInternal)
	})
	// If saving the key fails after the upload, the uploaded object is removed and the
	// previous photo is kept.
	t.Run("saving the key fails: the upload is cleaned up", func(t *testing.T) {
		e := newEnv(t)
		u := user(testUser, uuid.NewString()+".jpg")
		e.repo.EXPECT().GetByID(mock.Anything, u.ID).Return(u, nil).Once()
		put := e.expectPut(t, nil)
		e.expectSetKey(t, u, put, dbDown())
		e.store.EXPECT().Delete(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string) error {
			assert.Equal(t, put.key, key, "only the orphan upload is deleted")
			return nil
		}).Once()
		_, err := e.svc.SetAvatar(ctx, u.ID, pngImage(t, 64, 64, color.White))
		wantErr(t, err, apperr.KindUnknown, "")
	})
}

func TestRemoveAvatar(t *testing.T) {
	ctx := context.Background()

	t.Run("removes the key, then the object", func(t *testing.T) {
		e := newEnv(t)
		key := uuid.NewString() + ".jpg"
		u := user(testUser, key)
		cleared := *u
		cleared.AvatarKey = ""
		e.repo.EXPECT().GetByID(mock.Anything, u.ID).Return(u, nil).Once()
		setKey := e.repo.EXPECT().SetAvatarKey(mock.Anything, u.ID, "").Return(&cleared, nil).Once()
		del := e.store.EXPECT().Delete(mock.Anything, key).Return(nil).Once()
		mock.InOrder(setKey, del)

		got, err := e.svc.RemoveAvatar(ctx, u.ID)
		require.NoError(t, err)
		assert.Empty(t, got.AvatarKey)
	})
	t.Run("no photo: idempotent, the profile as is", func(t *testing.T) {
		e := newEnv(t)
		u := user(testUser, "")
		e.repo.EXPECT().GetByID(mock.Anything, u.ID).Return(u, nil).Once()
		got, err := e.svc.RemoveAvatar(ctx, u.ID)
		require.NoError(t, err)
		assert.Same(t, u, got)
	})
	t.Run("unknown user", func(t *testing.T) {
		e := newEnv(t)
		id := uuid.New()
		e.repo.EXPECT().GetByID(mock.Anything, id).Return(nil, userNotFound()).Once()
		_, err := e.svc.RemoveAvatar(ctx, id)
		wantErr(t, err, apperr.KindNotFound, "")
	})
	t.Run("saving fails: the object is kept", func(t *testing.T) {
		e := newEnv(t)
		u := user(testUser, uuid.NewString()+".jpg")
		e.repo.EXPECT().GetByID(mock.Anything, u.ID).Return(u, nil).Once()
		e.repo.EXPECT().SetAvatarKey(mock.Anything, u.ID, "").Return(nil, dbDown()).Once()
		_, err := e.svc.RemoveAvatar(ctx, u.ID)
		wantErr(t, err, apperr.KindUnknown, "")
	})
}

// closeRecorder is an object body that records Close.
type closeRecorder struct {
	io.Reader
	closed bool
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}

func TestOpenAvatar(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the stored JPEG and closes the body", func(t *testing.T) {
		e := newEnv(t)
		key := uuid.NewString() + ".jpg"
		data := []byte("jpeg bytes")
		body := &closeRecorder{Reader: bytes.NewReader(data)}
		e.store.EXPECT().Get(mock.Anything, key).
			Return(&blob.Object{ContentType: "image/jpeg", Size: int64(len(data)), Body: body}, nil).Once()

		av, err := e.svc.OpenAvatar(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, &users.Avatar{ContentType: "image/jpeg", Data: data}, av)
		assert.True(t, body.closed)
	})
	t.Run("malformed keys do not reach the store", func(t *testing.T) {
		e := newEnv(t)
		foreign := uuid.NewString() + ".jpg"
		for _, key := range []string{
			"", "../" + foreign, "secret.txt", strings.ToUpper(uuid.NewString()) + ".jpg",
			uuid.NewString() + ".png", uuid.NewString() + ".jpg/x",
		} {
			_, err := e.svc.OpenAvatar(ctx, key)
			wantErr(t, err, apperr.KindNotFound, service.MsgAvatarNotFound)
		}
	})
	t.Run("unknown (e.g. replaced) key", func(t *testing.T) {
		e := newEnv(t)
		key := uuid.NewString() + ".jpg"
		e.store.EXPECT().Get(mock.Anything, key).Return(nil, blob.ErrNotFound).Once()
		_, err := e.svc.OpenAvatar(ctx, key)
		wantErr(t, err, apperr.KindNotFound, service.MsgAvatarNotFound)
	})
	t.Run("store failure", func(t *testing.T) {
		e := newEnv(t)
		e.store.EXPECT().Get(mock.Anything, mock.Anything).Return(nil, errors.New("s3 down")).Once()
		_, err := e.svc.OpenAvatar(ctx, uuid.NewString()+".jpg")
		wantErr(t, err, apperr.KindUnknown, msgInternal)
	})
	t.Run("read failure", func(t *testing.T) {
		e := newEnv(t)
		body := &closeRecorder{Reader: iotest.ErrReader(errors.New("connection reset"))}
		e.store.EXPECT().Get(mock.Anything, mock.Anything).Return(&blob.Object{Body: body}, nil).Once()
		_, err := e.svc.OpenAvatar(ctx, uuid.NewString()+".jpg")
		wantErr(t, err, apperr.KindUnknown, msgInternal)
		assert.True(t, body.closed)
	})
	t.Run("oversized object is not ours", func(t *testing.T) {
		e := newEnv(t)
		body := &closeRecorder{Reader: bytes.NewReader(make([]byte, 1<<20+1))}
		e.store.EXPECT().Get(mock.Anything, mock.Anything).Return(&blob.Object{Body: body}, nil).Once()
		_, err := e.svc.OpenAvatar(ctx, uuid.NewString()+".jpg")
		wantErr(t, err, apperr.KindUnknown, msgInternal)
	})
}
