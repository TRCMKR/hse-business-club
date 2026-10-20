package bff

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	appmocks "github.com/trcmkr/hse-business-club/back/internal/applications/mocks"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	listingmocks "github.com/trcmkr/hse-business-club/back/internal/listings/mocks"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	authmocks "github.com/trcmkr/hse-business-club/back/internal/platform/auth/mocks"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	usermocks "github.com/trcmkr/hse-business-club/back/internal/users/mocks"
)

// The BFF only passes the rules of SPEC §2.3 through: the contexts own them. Unit
// tests here stub the ports with mockery mocks (ARCHITECTURE §6a) and check that a
// port error reaches the client with the right gRPC code and the same message, that
// requests are validated before any port call, and how responses are composed.
// A mock fails the test on any call without an expectation, so a test that sets no
// expectations on a port also asserts that the port is not called.

// Test data used across tests.
const (
	nameAnna      = "anna"
	nameBoris     = "boris"
	testPassword  = "demo1234"
	lookingForCTO = "CTO"
	titleFinTrack = "FinTrack"
	badID         = "bad"
	tagGo         = "Go"
	jpegType      = "image/jpeg"
	testAccessTTL = 15 * time.Minute
	testRefresh   = "refresh-token"
)

// caseInternal names subtests where a port fails with a plain error (500).
const caseInternal = "internal"

// Messages of the contexts (they own the real ones); the BFF passes them through.
const (
	cmsgLoginTaken      = "Логин уже занят"
	cmsgBadLogin        = "Неверный логин или пароль"
	cmsgUsername        = "Логин: от 3 до 32 символов"
	cmsgDisplayName     = "Имя: от 1 до 80 символов"
	cmsgAvatarFormat    = "Фото: поддерживаются JPEG, PNG и WebP"
	cmsgAvatarNotFound  = "Фото не найдено"
	cmsgTitle           = "Название: от 3 до 120 символов"
	cmsgStageRequired   = "Стадия: укажите стадию"
	cmsgLimit           = "Размер страницы: от 1 до 50"
	cmsgForbiddenEdit   = "Редактировать публикацию может только автор"
	cmsgListingClosed   = "Публикация закрыта"
	cmsgOwnListing      = "Нельзя откликнуться на свою публикацию"
	cmsgAlreadyApplied  = "Вы уже откликались на эту публикацию"
	cmsgMessage         = "Сообщение: от 1 до 2000 символов"
	cmsgDecided         = "Решение по отклику уже принято"
	cmsgForbiddenListAp = "Смотреть отклики может только автор"
	cmsgNoRights        = "Недостаточно прав"
)

// baseTime is the creation time of fixtures; later fixtures get later times.
var baseTime = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

// env is a BFF wired to mocks of all its ports.
type env struct {
	users    *usermocks.MockService
	listings *listingmocks.MockService
	apps     *appmocks.MockService
	issuer   *authmocks.MockIssuer
	bff      *Implementation
}

func newEnv(t *testing.T) *env {
	t.Helper()
	return newEnvOn(t)
}

// mockT is what mockery mocks need from a test.
type mockT interface {
	mock.TestingT
	Cleanup(func())
}

// newEnvOn is newEnv with mocks reporting to t (see serverT in gateway_test.go).
func newEnvOn(t mockT) *env {
	e := &env{
		users:    usermocks.NewMockService(t),
		listings: listingmocks.NewMockService(t),
		apps:     appmocks.NewMockService(t),
		issuer:   authmocks.NewMockIssuer(t),
	}
	e.bff = New(Deps{Users: e.users, Listings: e.listings, Applications: e.apps, Issuer: e.issuer, AccessTTL: testAccessTTL})
	return e
}

// as returns a context authenticated as id.
func as(id uuid.UUID) context.Context { return auth.WithUserID(context.Background(), id) }

// accessToken is what the mocked issuer returns for id.
func accessToken(id uuid.UUID) string { return "access:" + id.String() }

// expectIssue makes the issuer return accessToken(id) once.
func (e *env) expectIssue(id uuid.UUID) {
	e.issuer.EXPECT().Issue(id).Return(accessToken(id), nil).Once()
}

// newUser is a user fixture.
func newUser(username string) *users.User {
	return &users.User{
		ID: uuid.New(), Username: username, DisplayName: "Name " + username,
		CreatedAt: baseTime, UpdatedAt: baseTime,
	}
}

// newStartup is an active startup fixture by author.
func newStartup(author uuid.UUID, title string) *listings.Listing {
	return &listings.Listing{
		ID: uuid.New(), AuthorID: author, Kind: listings.KindStartup, Title: title,
		Description: "Description of " + title, LookingFor: lookingForCTO, Tags: []string{tagGo},
		Stage: listings.StageMVP, Status: listings.StatusActive, CreatedAt: baseTime, UpdatedAt: baseTime,
	}
}

// newVacancy is an active vacancy fixture by author.
func newVacancy(author uuid.UUID, title string) *listings.Listing {
	return &listings.Listing{
		ID: uuid.New(), AuthorID: author, Kind: listings.KindVacancy, Title: title,
		Description: "Description of " + title, LookingFor: "Designer", Compensation: "Доля",
		Status: listings.StatusActive, CreatedAt: baseTime, UpdatedAt: baseTime,
	}
}

// closed returns a closed copy of l.
func closed(l *listings.Listing) *listings.Listing {
	c := *l
	at := baseTime.Add(time.Hour)
	c.Status, c.ClosedAt, c.UpdatedAt = listings.StatusClosed, &at, at
	return &c
}

// newApp is a pending application fixture of applicant to l.
func newApp(l *listings.Listing, applicant uuid.UUID) *applications.Application {
	return &applications.Application{
		ID: uuid.New(), ListingID: l.ID, ListingAuthorID: l.AuthorID, ApplicantID: applicant,
		Message: "Хочу в команду", Status: applications.StatusPending, CreatedAt: baseTime, UpdatedAt: baseTime,
	}
}

// withStatus returns a copy of a in status s (decided_at is set for accepted/declined).
func withStatus(a *applications.Application, s applications.Status) *applications.Application {
	c := *a
	c.Status, c.UpdatedAt = s, baseTime.Add(time.Hour)
	if s == applications.StatusAccepted || s == applications.StatusDeclined {
		at := c.UpdatedAt
		c.DecidedAt = &at
	}
	return &c
}

// byID maps users by id, as Users.GetMany returns them.
func byID(us ...*users.User) map[uuid.UUID]*users.User {
	out := make(map[uuid.UUID]*users.User, len(us))
	for _, u := range us {
		out[u.ID] = u
	}
	return out
}

// listingsByID maps listings by id, as Listings.GetMany returns them.
func listingsByID(ls ...*listings.Listing) map[uuid.UUID]*listings.Listing {
	out := make(map[uuid.UUID]*listings.Listing, len(ls))
	for _, l := range ls {
		out[l.ID] = l
	}
	return out
}

// userIDs returns the ids of us.
func userIDs(us ...*users.User) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(us))
	for _, u := range us {
		out = append(out, u.ID)
	}
	return out
}

// sameIDs matches an id slice that holds exactly the want ids, each once, in any
// order: batch calls get every needed id and no duplicates.
func sameIDs(want ...uuid.UUID) any {
	return mock.MatchedBy(func(got []uuid.UUID) bool { return equalIDSets(got, want) })
}

func equalIDSets(got, want []uuid.UUID) bool {
	if len(got) != len(want) {
		return false
	}
	set := make(map[uuid.UUID]bool, len(want))
	for _, id := range want {
		set[id] = true
	}
	for _, id := range got {
		if !set[id] {
			return false // unknown or repeated
		}
		delete(set, id)
	}
	return true
}

// expectUsers expects times Users.GetMany calls for exactly us and returns them.
func (e *env) expectUsers(times int, us ...*users.User) {
	e.users.EXPECT().GetMany(mock.Anything, sameIDs(userIDs(us...)...)).Return(byID(us...), nil).Times(times)
}

// expectListings expects times Listings.GetMany calls for exactly ls and returns them.
func (e *env) expectListings(times int, ls ...*listings.Listing) {
	ids := make([]uuid.UUID, 0, len(ls))
	for _, l := range ls {
		ids = append(ids, l.ID)
	}
	e.listings.EXPECT().GetMany(mock.Anything, sameIDs(ids...)).Return(listingsByID(ls...), nil).Times(times)
}

// errOf is a context error of kind with msg.
func errOf(kind apperr.Kind, msg string) error { return apperr.New(kind, msg) }

// wantStatus asserts that err is a gRPC status with the code and message.
func wantStatus(t *testing.T, err error, code codes.Code, msg string) {
	t.Helper()
	st, ok := status.FromError(err)
	require.Truef(t, ok, "error %v is not a gRPC status", err)
	require.Equal(t, code, st.Code(), "code (message %q)", st.Message())
	require.Equal(t, msg, st.Message())
}

func wantOK(t *testing.T, err error) {
	t.Helper()
	require.NoError(t, err)
}
