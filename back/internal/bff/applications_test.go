package bff

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

const applyMessage = "Хочу в команду"

func appIDs(items []*hbcv1.Application) []string {
	out := make([]string, 0, len(items))
	for _, a := range items {
		out = append(out, a.GetId())
	}
	return out
}

func joinIDs(apps ...*applications.Application) []string {
	out := make([]string, 0, len(apps))
	for _, a := range apps {
		out = append(out, a.ID.String())
	}
	return out
}

func TestV1CreateApplication(t *testing.T) {
	t.Parallel()
	anna, boris := newUser(nameAnna), newUser(nameBoris)
	fin := newStartup(anna.ID, titleFinTrack)
	req := &hbcv1.V1CreateApplicationRequest{ListingId: fin.ID.String(), Message: applyMessage}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		app := newApp(fin, boris.ID)
		e.apps.EXPECT().Apply(mock.Anything, fin.ID, boris.ID, applyMessage).Return(app, nil).Once()
		e.expectListings(1, fin)
		e.expectUsers(1, boris, anna) // applicant and listing author

		a, err := e.bff.V1CreateApplication(as(boris.ID), req)
		wantOK(t, err)
		if a.GetId() != app.ID.String() || a.GetStatus() != hbcv1.ApplicationStatus_APPLICATION_STATUS_PENDING || a.GetMessage() != applyMessage ||
			a.GetApplicant().GetUsername() != nameBoris || a.GetDecidedAt() != nil || a.GetCreatedAt() == nil || a.GetUpdatedAt() == nil {
			t.Fatalf("application = %v", a)
		}
		if s := a.GetListing(); s.GetId() != fin.ID.String() || s.GetTitle() != titleFinTrack || s.GetKind() != hbcv1.ListingKind_LISTING_KIND_STARTUP ||
			s.GetStatus() != hbcv1.ListingStatus_LISTING_STATUS_ACTIVE || s.GetAuthor().GetUsername() != nameAnna {
			t.Fatalf("listing summary = %v", s)
		}
	})

	// The rules of applying belong to the context (SPEC §2.3); the BFF passes the
	// error with its code and message.
	for _, tt := range []struct {
		name string
		err  error
		code codes.Code
		msg  string
	}{
		{"repeated", errOf(apperr.KindConflict, cmsgAlreadyApplied), codes.AlreadyExists, cmsgAlreadyApplied},
		{"own listing", errOf(apperr.KindPrecondition, cmsgOwnListing), codes.FailedPrecondition, cmsgOwnListing},
		{"closed listing", errOf(apperr.KindPrecondition, cmsgListingClosed), codes.FailedPrecondition, cmsgListingClosed},
		{"bad message", apperr.Validation("message", cmsgMessage), codes.InvalidArgument, cmsgMessage},
		{"unknown listing", errOf(apperr.KindNotFound, msgListingNotFound), codes.NotFound, msgListingNotFound},
		{caseInternal, errors.New(errDB), codes.Internal, msgInternal},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.apps.EXPECT().Apply(mock.Anything, fin.ID, boris.ID, applyMessage).Return(nil, tt.err).Once()

			_, err := e.bff.V1CreateApplication(as(boris.ID), req)
			wantStatus(t, err, tt.code, tt.msg)
		})
	}

	// After withdraw the context creates a new record; the BFF returns it as is.
	t.Run("re-application", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		again := newApp(fin, boris.ID)
		e.apps.EXPECT().Apply(mock.Anything, fin.ID, boris.ID, applyMessage).Return(again, nil).Once()
		e.expectListings(1, fin)
		e.expectUsers(1, boris, anna)

		a, err := e.bff.V1CreateApplication(as(boris.ID), req)
		wantOK(t, err)
		require.Equal(t, again.ID.String(), a.GetId())
	})

	t.Run("bad listing id", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t) // no expectations: ports are not called
		_, err := e.bff.V1CreateApplication(as(boris.ID), &hbcv1.V1CreateApplicationRequest{ListingId: badID, Message: "x"})
		wantStatus(t, err, codes.NotFound, msgListingNotFound)
	})

	t.Run("compose fails", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.apps.EXPECT().Apply(mock.Anything, fin.ID, boris.ID, applyMessage).Return(newApp(fin, boris.ID), nil).Once()
		e.listings.EXPECT().GetMany(mock.Anything, mock.Anything).Return(nil, errors.New(errDB)).Once()

		_, err := e.bff.V1CreateApplication(as(boris.ID), req)
		wantStatus(t, err, codes.Internal, msgInternal)
	})
}

func TestV1ListListingApplications(t *testing.T) {
	t.Parallel()
	anna, boris, vera := newUser(nameAnna), newUser(nameBoris), newUser(nameVera)
	fin := newStartup(anna.ID, titleFinTrack)
	first := newApp(fin, boris.ID)
	second := withStatus(newApp(fin, vera.ID), applications.StatusDeclined)
	ctx := as(anna.ID)

	t.Run("all", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.apps.EXPECT().ListByListing(mock.Anything, fin.ID, anna.ID, applications.Status("")).
			Return([]*applications.Application{first, second}, nil).Once()
		e.expectListings(1, fin) // one listing for all its applications
		e.expectUsers(1, boris, vera, anna)

		resp, err := e.bff.V1ListListingApplications(ctx, &hbcv1.V1ListListingApplicationsRequest{ListingId: fin.ID.String()})
		wantOK(t, err)
		items := resp.GetItems()
		require.Equal(t, joinIDs(first, second), appIDs(items)) // port order
		if items[0].GetApplicant().GetUsername() != nameBoris || items[1].GetApplicant().GetUsername() != nameVera ||
			items[0].GetListing().GetAuthor().GetUsername() != nameAnna || items[1].GetListing().GetId() != fin.ID.String() {
			t.Fatalf("composition = %v", items)
		}
	})

	t.Run("status filter", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.apps.EXPECT().ListByListing(mock.Anything, fin.ID, anna.ID, applications.StatusDeclined).
			Return([]*applications.Application{second}, nil).Once()
		e.expectListings(1, fin)
		e.expectUsers(1, vera, anna)

		resp, err := e.bff.V1ListListingApplications(ctx, &hbcv1.V1ListListingApplicationsRequest{
			ListingId: fin.ID.String(), Status: hbcv1.ApplicationStatus_APPLICATION_STATUS_DECLINED,
		})
		wantOK(t, err)
		require.Len(t, resp.GetItems(), 1)
		require.Equal(t, hbcv1.ApplicationStatus_APPLICATION_STATUS_DECLINED, resp.GetItems()[0].GetStatus())
		require.NotNil(t, resp.GetItems()[0].GetDecidedAt())
	})

	// Only the author sees applications: the context answers 403.
	t.Run("not the author", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.apps.EXPECT().ListByListing(mock.Anything, fin.ID, boris.ID, applications.Status("")).
			Return(nil, errOf(apperr.KindForbidden, cmsgForbiddenListAp)).Once()

		_, err := e.bff.V1ListListingApplications(as(boris.ID), &hbcv1.V1ListListingApplicationsRequest{ListingId: fin.ID.String()})
		wantStatus(t, err, codes.PermissionDenied, cmsgForbiddenListAp)
	})

	// Shape errors: no port calls (no expectations).
	t.Run("bad listing id", func(t *testing.T) {
		t.Parallel()
		_, err := newEnv(t).bff.V1ListListingApplications(ctx, &hbcv1.V1ListListingApplicationsRequest{ListingId: badID})
		wantStatus(t, err, codes.NotFound, msgListingNotFound)
	})
	t.Run("bad status", func(t *testing.T) {
		t.Parallel()
		_, err := newEnv(t).bff.V1ListListingApplications(ctx, &hbcv1.V1ListListingApplicationsRequest{ListingId: fin.ID.String(), Status: 10})
		wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	})
}

// transition describes one of Accept, Decline, Withdraw: the RPC and the port it calls.
type transition struct {
	name   string
	status applications.Status
	want   hbcv1.ApplicationStatus
	rpc    func(b *Implementation, ctx context.Context, req *hbcv1.V1ApplicationIDRequest) (*hbcv1.Application, error)
	// expect makes the port called with (id, actor) return (a, err) times times.
	expect func(e *env, id, actor uuid.UUID, a *applications.Application, err error, times int)
}

var transitions = []transition{
	{
		name: "accept", status: applications.StatusAccepted, want: hbcv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED,
		rpc: (*Implementation).V1AcceptApplication,
		expect: func(e *env, id, actor uuid.UUID, a *applications.Application, err error, times int) {
			e.apps.EXPECT().Accept(mock.Anything, id, actor).Return(a, err).Times(times)
		},
	},
	{
		name: "decline", status: applications.StatusDeclined, want: hbcv1.ApplicationStatus_APPLICATION_STATUS_DECLINED,
		rpc: (*Implementation).V1DeclineApplication,
		expect: func(e *env, id, actor uuid.UUID, a *applications.Application, err error, times int) {
			e.apps.EXPECT().Decline(mock.Anything, id, actor).Return(a, err).Times(times)
		},
	},
	{
		name: "withdraw", status: applications.StatusWithdrawn, want: hbcv1.ApplicationStatus_APPLICATION_STATUS_WITHDRAWN,
		rpc: (*Implementation).V1WithdrawApplication,
		expect: func(e *env, id, actor uuid.UUID, a *applications.Application, err error, times int) {
			e.apps.EXPECT().Withdraw(mock.Anything, id, actor).Return(a, err).Times(times)
		},
	},
}

// TestV1ApplicationTransitions covers accept, decline (listing author) and withdraw
// (applicant): the decision rules are the context's (SPEC §2.3), the BFF passes
// the caller as the actor, composes the result and passes errors through.
func TestV1ApplicationTransitions(t *testing.T) {
	t.Parallel()
	anna, boris, vera := newUser(nameAnna), newUser(nameBoris), newUser(nameVera)
	fin := newStartup(anna.ID, titleFinTrack)
	app := newApp(fin, boris.ID)
	req := &hbcv1.V1ApplicationIDRequest{Id: app.ID.String()}

	for _, tr := range transitions {
		// The side allowed to act and the other side of the application.
		actor, other := anna, boris
		if tr.status == applications.StatusWithdrawn {
			actor, other = boris, anna
		}

		t.Run(tr.name, func(t *testing.T) {
			t.Parallel()

			// Idempotent: a repeated call returns the same application.
			t.Run("idempotent", func(t *testing.T) {
				t.Parallel()
				e := newEnv(t)
				done := withStatus(app, tr.status)
				tr.expect(e, app.ID, actor.ID, done, nil, 2)
				e.expectListings(2, fin)
				e.expectUsers(2, boris, anna)

				for range 2 {
					a, err := tr.rpc(e.bff, as(actor.ID), req)
					wantOK(t, err)
					require.Equal(t, app.ID.String(), a.GetId())
					require.Equal(t, tr.want, a.GetStatus())
					require.Equal(t, tr.status != applications.StatusWithdrawn, a.GetDecidedAt() != nil)
					require.Equal(t, nameBoris, a.GetApplicant().GetUsername())
					require.Equal(t, nameAnna, a.GetListing().GetAuthor().GetUsername())
				}
			})

			for _, tt := range []struct {
				name   string
				caller *users.User
				err    error
				code   codes.Code
				msg    string
			}{
				// A decision is already made: accept after decline and so on.
				{"decided", actor, errOf(apperr.KindPrecondition, cmsgDecided), codes.FailedPrecondition, cmsgDecided},
				// The other side of the application: 403.
				{"other side", other, errOf(apperr.KindForbidden, cmsgNoRights), codes.PermissionDenied, cmsgNoRights},
				// A stranger: the application is not disclosed, 404.
				{"stranger", vera, errOf(apperr.KindNotFound, msgApplicationNotFound), codes.NotFound, msgApplicationNotFound},
				{caseInternal, actor, errors.New(errDB), codes.Internal, msgInternal},
			} {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					e := newEnv(t)
					tr.expect(e, app.ID, tt.caller.ID, nil, tt.err, 1)

					_, err := tr.rpc(e.bff, as(tt.caller.ID), req)
					wantStatus(t, err, tt.code, tt.msg)
				})
			}

			// Malformed and empty ids: 404 without port calls (no expectations).
			t.Run("bad id", func(t *testing.T) {
				t.Parallel()
				e := newEnv(t)
				for _, id := range []string{badID, ""} {
					_, err := tr.rpc(e.bff, as(actor.ID), &hbcv1.V1ApplicationIDRequest{Id: id})
					wantStatus(t, err, codes.NotFound, msgApplicationNotFound)
				}
			})
		})
	}
}

func TestV1ListSentAndReceivedApplications(t *testing.T) {
	t.Parallel()
	anna, boris, vera := newUser(nameAnna), newUser(nameBoris), newUser(nameVera)
	fin := newStartup(anna.ID, titleFinTrack)
	job := newVacancy(anna.ID, "Designer")
	other := newStartup(vera.ID, "Other")
	a1 := withStatus(newApp(fin, boris.ID), applications.StatusAccepted)
	a2, a3, a4 := newApp(job, boris.ID), newApp(other, boris.ID), newApp(job, vera.ID)

	t.Run("sent", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.apps.EXPECT().ListByApplicant(mock.Anything, boris.ID, applications.Status("")).
			Return([]*applications.Application{a3, a2, a1}, nil).Once()
		e.expectListings(1, other, job, fin)
		e.expectUsers(1, boris, vera, anna) // applicant and listing authors, each once

		resp, err := e.bff.V1ListSentApplications(as(boris.ID), &hbcv1.V1ListSentApplicationsRequest{})
		wantOK(t, err)
		require.Equal(t, joinIDs(a3, a2, a1), appIDs(resp.GetItems())) // port order
		for _, a := range resp.GetItems() {
			if a.GetApplicant().GetUsername() != nameBoris || a.GetListing().GetAuthor() == nil {
				t.Fatalf("composition = %v", a)
			}
		}
		require.Equal(t, nameVera, resp.GetItems()[0].GetListing().GetAuthor().GetUsername())
	})

	t.Run("sent with status", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.apps.EXPECT().ListByApplicant(mock.Anything, boris.ID, applications.StatusAccepted).
			Return([]*applications.Application{a1}, nil).Once()
		e.expectListings(1, fin)
		e.expectUsers(1, boris, anna)

		resp, err := e.bff.V1ListSentApplications(as(boris.ID), &hbcv1.V1ListSentApplicationsRequest{
			Status: hbcv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED,
		})
		wantOK(t, err)
		require.Equal(t, joinIDs(a1), appIDs(resp.GetItems()))
	})

	t.Run("received", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.apps.EXPECT().ListReceived(mock.Anything, anna.ID, applications.Status("")).
			Return([]*applications.Application{a4, a2, a1}, nil).Once()
		e.expectListings(1, job, fin)
		e.expectUsers(1, vera, boris, anna)

		resp, err := e.bff.V1ListReceivedApplications(as(anna.ID), &hbcv1.V1ListReceivedApplicationsRequest{})
		wantOK(t, err)
		require.Equal(t, joinIDs(a4, a2, a1), appIDs(resp.GetItems()))
		require.Equal(t, nameVera, resp.GetItems()[0].GetApplicant().GetUsername())
	})

	t.Run("received with status", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.apps.EXPECT().ListReceived(mock.Anything, anna.ID, applications.StatusPending).
			Return([]*applications.Application{a4, a2}, nil).Once()
		e.expectListings(1, job)
		e.expectUsers(1, vera, boris, anna)

		resp, err := e.bff.V1ListReceivedApplications(as(anna.ID), &hbcv1.V1ListReceivedApplicationsRequest{
			Status: hbcv1.ApplicationStatus_APPLICATION_STATUS_PENDING,
		})
		wantOK(t, err)
		require.Equal(t, joinIDs(a4, a2), appIDs(resp.GetItems()))
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.apps.EXPECT().ListReceived(mock.Anything, boris.ID, applications.Status("")).Return(nil, nil).Once()
		// Nothing to compose: no other port calls.

		resp, err := e.bff.V1ListReceivedApplications(as(boris.ID), &hbcv1.V1ListReceivedApplicationsRequest{})
		wantOK(t, err)
		require.NotNil(t, resp.GetItems())
		require.Empty(t, resp.GetItems())
	})

	t.Run("bad status", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t) // no expectations: ports are not called
		_, err := e.bff.V1ListSentApplications(as(boris.ID), &hbcv1.V1ListSentApplicationsRequest{Status: 99})
		wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
		_, err = e.bff.V1ListReceivedApplications(as(boris.ID), &hbcv1.V1ListReceivedApplicationsRequest{Status: 99})
		wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	})
}

// TestApplicationListsBatches: composing lists of applications takes two port calls
// (Listings.GetMany, Users.GetMany) for any number of items (B4).
func TestApplicationListsBatches(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 25} {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			anna, shared := newUser(nameAnna), newUser("shared")
			people := []*users.User{anna, shared}
			var (
				ls   []*listings.Listing
				apps []*applications.Application
			)
			for i := range n {
				l := newStartup(anna.ID, fmt.Sprintf("Listing %d", i))
				u := newUser(fmt.Sprintf("user%d", i))
				ls, people = append(ls, l), append(people, u)
				apps = append(apps, newApp(l, u.ID), newApp(l, shared.ID)) // shared applies everywhere
			}
			e.apps.EXPECT().ListReceived(mock.Anything, anna.ID, applications.Status("")).Return(apps, nil).Once()
			e.expectListings(1, ls...)  // each listing once
			e.expectUsers(1, people...) // each applicant and author once

			resp, err := e.bff.V1ListReceivedApplications(as(anna.ID), &hbcv1.V1ListReceivedApplicationsRequest{})
			wantOK(t, err)
			require.Equal(t, joinIDs(apps...), appIDs(resp.GetItems()))

			e.apps.AssertNumberOfCalls(t, "ListReceived", 1)
			e.listings.AssertNumberOfCalls(t, "GetMany", 1)
			e.users.AssertNumberOfCalls(t, "GetMany", 1)
			for _, a := range resp.GetItems() {
				require.NotNil(t, a.GetListing().GetAuthor())
				require.NotNil(t, a.GetApplicant())
			}
		})
	}
}

func TestApplicationsInternalError(t *testing.T) {
	t.Parallel()
	anna, boris := newUser(nameAnna), newUser(nameBoris)
	fin := newStartup(anna.ID, titleFinTrack)
	app := newApp(fin, boris.ID)
	boom := errors.New(errDB)

	for _, tt := range []struct {
		name   string
		expect func(e *env)
	}{
		{"list", func(e *env) {
			e.apps.EXPECT().ListByApplicant(mock.Anything, boris.ID, applications.Status("")).Return(nil, boom).Once()
		}},
		{"listings batch", func(e *env) {
			e.apps.EXPECT().ListByApplicant(mock.Anything, boris.ID, applications.Status("")).Return([]*applications.Application{app}, nil).Once()
			e.listings.EXPECT().GetMany(mock.Anything, mock.Anything).Return(nil, boom).Once()
		}},
		{"users batch", func(e *env) {
			e.apps.EXPECT().ListByApplicant(mock.Anything, boris.ID, applications.Status("")).Return([]*applications.Application{app}, nil).Once()
			e.expectListings(1, fin)
			e.users.EXPECT().GetMany(mock.Anything, mock.Anything).Return(nil, boom).Once()
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			tt.expect(e)
			_, err := e.bff.V1ListSentApplications(as(boris.ID), &hbcv1.V1ListSentApplicationsRequest{})
			wantStatus(t, err, codes.Internal, msgInternal)
		})
	}
}

// TestApplicationsMissingInBatch: a listing or user absent from a batch result
// leaves the field null instead of failing the response.
func TestApplicationsMissingInBatch(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris := newUser(nameAnna), newUser(nameBoris)
	fin := newStartup(anna.ID, titleFinTrack)
	app := newApp(fin, boris.ID)
	e.apps.EXPECT().ListByApplicant(mock.Anything, boris.ID, applications.Status("")).Return([]*applications.Application{app}, nil).Once()
	e.listings.EXPECT().GetMany(mock.Anything, []uuid.UUID{fin.ID}).Return(listingsByID(), nil).Once()
	e.users.EXPECT().GetMany(mock.Anything, []uuid.UUID{boris.ID}).Return(byID(), nil).Once()

	resp, err := e.bff.V1ListSentApplications(as(boris.ID), &hbcv1.V1ListSentApplicationsRequest{})
	wantOK(t, err)
	require.Len(t, resp.GetItems(), 1)
	require.Nil(t, resp.GetItems()[0].GetListing())
	require.Nil(t, resp.GetItems()[0].GetApplicant())
}
