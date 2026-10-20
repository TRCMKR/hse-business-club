package bff

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/users"
)

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
)

// env is a BFF wired to in-memory fakes.
type env struct {
	calls    *calls
	users    *fakeUsers
	listings *fakeListings
	apps     *fakeApplications
	bff      *Implementation
}

func newEnv(t *testing.T) *env {
	t.Helper()
	c, cl := &calls{}, &clock{}
	ls := newFakeListings(c, cl)
	e := &env{calls: c, users: newFakeUsers(c, cl), listings: ls, apps: newFakeApplications(c, cl, ls)}
	e.bff = New(Deps{Users: e.users, Listings: e.listings, Applications: e.apps, Issuer: fakeIssuer{}, AccessTTL: testAccessTTL})
	return e
}

// as returns a context authenticated as id.
func as(id uuid.UUID) context.Context { return auth.WithUserID(context.Background(), id) }

// user registers a member directly in the fake (password testPassword).
func (e *env) user(t *testing.T, username string) *users.User {
	t.Helper()
	u, err := e.users.Register(context.Background(), users.RegisterParams{Username: username, Password: testPassword, DisplayName: "Name " + username})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (e *env) startup(t *testing.T, author uuid.UUID, title string) *listings.Listing {
	t.Helper()
	l, err := e.listings.Create(context.Background(), listings.CreateParams{
		AuthorID: author, Kind: listings.KindStartup, Title: title, Description: "Description of " + title,
		LookingFor: lookingForCTO, Tags: []string{tagGo}, Stage: listings.StageMVP,
	})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func (e *env) vacancy(t *testing.T, author uuid.UUID, title string) *listings.Listing {
	t.Helper()
	l, err := e.listings.Create(context.Background(), listings.CreateParams{
		AuthorID: author, Kind: listings.KindVacancy, Title: title, Description: "Description of " + title,
		LookingFor: "Designer", Compensation: "Доля",
	})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func (e *env) closeListing(t *testing.T, l *listings.Listing) {
	t.Helper()
	if _, err := e.listings.Close(context.Background(), l.ID, l.AuthorID); err != nil {
		t.Fatal(err)
	}
}

func (e *env) apply(t *testing.T, l *listings.Listing, applicant uuid.UUID) *applications.Application {
	t.Helper()
	a, err := e.apps.Apply(context.Background(), l.ID, applicant, "Хочу в команду")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (e *env) setStatus(t *testing.T, a *applications.Application, s applications.Status) {
	t.Helper()
	var err error
	switch s {
	case applications.StatusAccepted:
		_, err = e.apps.Accept(context.Background(), a.ID, a.ListingAuthorID)
	case applications.StatusDeclined:
		_, err = e.apps.Decline(context.Background(), a.ID, a.ListingAuthorID)
	case applications.StatusWithdrawn:
		_, err = e.apps.Withdraw(context.Background(), a.ID, a.ApplicantID)
	case applications.StatusPending:
	}
	if err != nil {
		t.Fatal(err)
	}
}

// wantStatus asserts that err is a gRPC status with the code and message.
func wantStatus(t *testing.T, err error, code codes.Code, msg string) {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error %v is not a gRPC status", err)
	}
	if st.Code() != code || st.Message() != msg {
		t.Fatalf("got %s %q, want %s %q", st.Code(), st.Message(), code, msg)
	}
}

func wantOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// noCalls asserts that no port was called since the last reset.
func (e *env) noCalls(t *testing.T) {
	t.Helper()
	if c := e.calls.snapshot(); len(c) != 0 {
		t.Fatalf("ports were called: %v", c)
	}
}
