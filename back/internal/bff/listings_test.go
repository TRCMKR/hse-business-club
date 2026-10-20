package bff

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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

const (
	nameVera     = "vera"
	compensation = "Доля"
	errDB        = "pgx: broken pipe"
)

// feedFilter is the ListActive filter of a request without page/limit (defaults 1/20).
func feedFilter(kind listings.Kind) listings.ListFilter {
	return listings.ListFilter{Kind: kind, Page: defaultPage, Limit: defaultLimit}
}

func listingPage(f listings.ListFilter, total int, ls ...*listings.Listing) *listings.Page {
	return &listings.Page{Items: ls, Total: total, Page: f.Page, Limit: f.Limit}
}

func noStats() map[uuid.UUID]applications.Stats { return map[uuid.UUID]applications.Stats{} }

func listingIDs(items []*hbcv1.Listing) []string {
	out := make([]string, 0, len(items))
	for _, l := range items {
		out = append(out, l.GetId())
	}
	return out
}

func TestV1ListListingsComposition(t *testing.T) {
	t.Parallel()
	anna, boris := newUser(nameAnna), newUser(nameBoris)
	fin := newStartup(anna.ID, titleFinTrack)
	own := newVacancy(boris.ID, "Boris vacancy")
	own.CreatedAt = baseTime.Add(time.Minute)
	app := newApp(fin, boris.ID)

	t.Run("own and foreign", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := feedFilter("")
		e.listings.EXPECT().ListActive(mock.Anything, f).Return(listingPage(f, 2, own, fin), nil).Once()
		e.expectUsers(1, boris, anna) // authors and me (the applicant of myApplication)
		e.apps.EXPECT().GetMineForListings(mock.Anything, boris.ID, []uuid.UUID{fin.ID}).
			Return(map[uuid.UUID]*applications.Application{fin.ID: app}, nil).Once()
		// No key for own: zero stats, not null.
		e.apps.EXPECT().StatsForListings(mock.Anything, []uuid.UUID{own.ID}).Return(noStats(), nil).Once()

		resp, err := e.bff.V1ListListings(as(boris.ID), &hbcv1.V1ListListingsRequest{})
		wantOK(t, err)
		require.EqualValues(t, 2, resp.GetTotal())
		require.EqualValues(t, defaultPage, resp.GetPage())
		require.EqualValues(t, defaultLimit, resp.GetLimit())
		require.Equal(t, []string{own.ID.String(), fin.ID.String()}, listingIDs(resp.GetItems())) // port order
		mine, foreign := resp.GetItems()[0], resp.GetItems()[1]

		require.True(t, mine.GetIsMine())
		require.Nil(t, mine.GetMyApplication())
		require.NotNil(t, mine.GetStats())
		require.Zero(t, mine.GetStats().GetTotal())
		if mine.GetAuthor().GetUsername() != nameBoris || mine.GetKind() != hbcv1.ListingKind_LISTING_KIND_VACANCY ||
			mine.GetCompensation() != compensation || mine.GetStage() != hbcv1.StartupStage_STARTUP_STAGE_UNSPECIFIED {
			t.Fatalf("own listing = %v", mine)
		}

		require.False(t, foreign.GetIsMine())
		require.Nil(t, foreign.GetStats())
		if foreign.GetAuthor().GetUsername() != nameAnna || foreign.GetStage() != hbcv1.StartupStage_STARTUP_STAGE_MVP ||
			foreign.GetStatus() != hbcv1.ListingStatus_LISTING_STATUS_ACTIVE || foreign.GetClosedAt() != nil ||
			foreign.GetLookingFor() != lookingForCTO || strings.Join(foreign.GetTags(), ",") != tagGo ||
			foreign.GetCreatedAt().AsTime() != fin.CreatedAt || foreign.GetUpdatedAt().AsTime() != fin.UpdatedAt {
			t.Fatalf("foreign listing = %v", foreign)
		}
		my := foreign.GetMyApplication()
		if my.GetId() != app.ID.String() || my.GetStatus() != hbcv1.ApplicationStatus_APPLICATION_STATUS_PENDING ||
			my.GetApplicant().GetUsername() != nameBoris || my.GetListing().GetId() != fin.ID.String() ||
			my.GetListing().GetAuthor().GetUsername() != nameAnna || my.GetDecidedAt() != nil {
			t.Fatalf("myApplication = %v", my)
		}
	})

	// No application of mine (none or withdrawn: the context leaves them out): null
	// myApplication; no own listings: StatsForListings is not called.
	t.Run("no application", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := feedFilter(listings.KindStartup)
		e.listings.EXPECT().ListActive(mock.Anything, f).Return(listingPage(f, 1, fin), nil).Once()
		e.expectUsers(1, anna, boris)
		e.apps.EXPECT().GetMineForListings(mock.Anything, boris.ID, []uuid.UUID{fin.ID}).
			Return(map[uuid.UUID]*applications.Application{}, nil).Once()

		resp, err := e.bff.V1ListListings(as(boris.ID), &hbcv1.V1ListListingsRequest{Kind: hbcv1.ListingKind_LISTING_KIND_STARTUP})
		wantOK(t, err)
		require.Len(t, resp.GetItems(), 1)
		require.Nil(t, resp.GetItems()[0].GetMyApplication())
	})

	// Only own listings: GetMineForListings is not called and me is asked for once.
	t.Run("only own", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := feedFilter("")
		e.listings.EXPECT().ListActive(mock.Anything, f).Return(listingPage(f, 1, own), nil).Once()
		e.expectUsers(1, boris)
		e.apps.EXPECT().StatsForListings(mock.Anything, []uuid.UUID{own.ID}).
			Return(map[uuid.UUID]applications.Stats{own.ID: {Pending: 2, Total: 2}}, nil).Once()

		resp, err := e.bff.V1ListListings(as(boris.ID), &hbcv1.V1ListListingsRequest{})
		wantOK(t, err)
		require.EqualValues(t, 2, resp.GetItems()[0].GetStats().GetPending())
	})
}

func TestV1ListListingsParams(t *testing.T) {
	t.Parallel()
	anna := newUser(nameAnna)
	alpha, beta, gamma := newStartup(anna.ID, "Alpha"), newStartup(anna.ID, "Beta"), newStartup(anna.ID, "Gamma")
	ctx := as(anna.ID)

	// kind, query, page and limit go to the context as is; the response keeps the
	// port's order and paging.
	t.Run("passed to the context", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := listings.ListFilter{Kind: listings.KindStartup, Query: "beta", Page: 2, Limit: 3}
		e.listings.EXPECT().ListActive(mock.Anything, f).Return(listingPage(f, 6, gamma, alpha, beta), nil).Once()
		e.expectUsers(1, anna)
		e.apps.EXPECT().StatsForListings(mock.Anything, sameIDs(gamma.ID, alpha.ID, beta.ID)).Return(noStats(), nil).Once()

		resp, err := e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{
			Kind: hbcv1.ListingKind_LISTING_KIND_STARTUP, Query: "beta", Page: 2, Limit: 3,
		})
		wantOK(t, err)
		require.EqualValues(t, 6, resp.GetTotal())
		require.EqualValues(t, 2, resp.GetPage())
		require.EqualValues(t, 3, resp.GetLimit())
		require.Equal(t, []string{gamma.ID.String(), alpha.ID.String(), beta.ID.String()}, listingIDs(resp.GetItems()))
	})

	t.Run("empty page", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := feedFilter(listings.KindVacancy)
		e.listings.EXPECT().ListActive(mock.Anything, f).Return(listingPage(f, 0), nil).Once()
		// Nothing to compose: no other port calls.

		resp, err := e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{Kind: hbcv1.ListingKind_LISTING_KIND_VACANCY})
		wantOK(t, err)
		require.NotNil(t, resp.GetItems())
		require.Empty(t, resp.GetItems())
	})

	// Page/limit ranges are the context's rule: 51 is passed and its error returned.
	t.Run("limit out of range", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().ListActive(mock.Anything, listings.ListFilter{Page: defaultPage, Limit: 51}).
			Return(nil, apperr.Validation("limit", cmsgLimit)).Once()

		_, err := e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{Limit: 51})
		wantStatus(t, err, codes.InvalidArgument, cmsgLimit)
	})

	t.Run("bad kind", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t) // no expectations: ports are not called
		_, err := e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{Kind: 99})
		wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	})

	t.Run(caseInternal, func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().ListActive(mock.Anything, mock.Anything).Return(nil, errors.New(errDB)).Once()

		_, err := e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{})
		wantStatus(t, err, codes.Internal, msgInternal)
	})

	t.Run("compose fails", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		f := feedFilter("")
		e.listings.EXPECT().ListActive(mock.Anything, f).Return(listingPage(f, 1, alpha), nil).Once()
		e.users.EXPECT().GetMany(mock.Anything, mock.Anything).Return(nil, errors.New(errDB)).Once()

		_, err := e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{})
		wantStatus(t, err, codes.Internal, msgInternal)
	})
}

// TestV1ListListingsBatches: the number of port calls does not depend on the page
// size (B4): one ListActive, then one batch per port for any number of items.
func TestV1ListListingsBatches(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 25} { // 2 and 50 items
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			me := newUser("myself")
			people := []*users.User{me}
			var (
				items             []*listings.Listing
				foreignIDs, myIDs []uuid.UUID
			)
			mine := map[uuid.UUID]*applications.Application{}
			stats := noStats()
			for i := range n {
				author := newUser(fmt.Sprintf("author%d", i))
				people = append(people, author)
				foreign, own := newStartup(author.ID, "Foreign listing"), newStartup(me.ID, "My listing")
				items = append(items, foreign, own)
				foreignIDs, myIDs = append(foreignIDs, foreign.ID), append(myIDs, own.ID)
				if i%2 == 0 {
					mine[foreign.ID] = newApp(foreign, me.ID)
					stats[own.ID] = applications.Stats{Pending: 1, Total: 1}
				}
			}

			f := listings.ListFilter{Page: defaultPage, Limit: 50}
			e.listings.EXPECT().ListActive(mock.Anything, f).Return(listingPage(f, len(items), items...), nil).Once()
			// Every author once (me is the author of n listings and the applicant).
			e.users.EXPECT().GetMany(mock.Anything, sameIDs(userIDs(people...)...)).Return(byID(people...), nil).Once()
			e.apps.EXPECT().GetMineForListings(mock.Anything, me.ID, sameIDs(foreignIDs...)).Return(mine, nil).Once()
			e.apps.EXPECT().StatsForListings(mock.Anything, sameIDs(myIDs...)).Return(stats, nil).Once()

			resp, err := e.bff.V1ListListings(as(me.ID), &hbcv1.V1ListListingsRequest{Limit: 50})
			wantOK(t, err)
			require.Len(t, resp.GetItems(), 2*n)

			e.listings.AssertNumberOfCalls(t, "ListActive", 1)
			e.users.AssertNumberOfCalls(t, "GetMany", 1)
			e.apps.AssertNumberOfCalls(t, "GetMineForListings", 1)
			e.apps.AssertNumberOfCalls(t, "StatsForListings", 1)

			withApp := 0
			for i, l := range resp.GetItems() {
				require.Equal(t, items[i].ID.String(), l.GetId())
				require.NotNil(t, l.GetAuthor())
				require.Equal(t, i%2 == 1, l.GetIsMine())
				require.Equal(t, l.GetIsMine(), l.GetStats() != nil)
				if l.GetMyApplication() != nil {
					withApp++
				}
			}
			require.Equal(t, len(mine), withApp)
		})
	}
}

func TestV1CreateListing(t *testing.T) {
	t.Parallel()
	anna := newUser(nameAnna)
	ctx := as(anna.ID)

	t.Run("startup", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		l := newStartup(anna.ID, titleFinTrack)
		l.Stage, l.Tags = listings.StageIdea, []string{tagGo, lookingForCTO}
		e.listings.EXPECT().Create(mock.Anything, listings.CreateParams{
			AuthorID: anna.ID, Kind: listings.KindStartup, Title: titleFinTrack, Description: l.Description,
			LookingFor: lookingForCTO, Tags: []string{tagGo, lookingForCTO}, Stage: listings.StageIdea,
		}).Return(l, nil).Once()
		e.expectUsers(1, anna)
		e.apps.EXPECT().StatsForListings(mock.Anything, []uuid.UUID{l.ID}).Return(noStats(), nil).Once()

		got, err := e.bff.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{
			Kind: hbcv1.ListingKind_LISTING_KIND_STARTUP, Title: titleFinTrack, Description: l.Description,
			LookingFor: lookingForCTO, Tags: []string{tagGo, lookingForCTO}, Stage: hbcv1.StartupStage_STARTUP_STAGE_IDEA,
		})
		wantOK(t, err)
		if !got.GetIsMine() || got.GetAuthor().GetId() != anna.ID.String() || got.GetStats() == nil || got.GetMyApplication() != nil ||
			got.GetStage() != hbcv1.StartupStage_STARTUP_STAGE_IDEA || got.GetStatus() != hbcv1.ListingStatus_LISTING_STATUS_ACTIVE ||
			got.GetClosedAt() != nil || got.GetTitle() != titleFinTrack || got.GetId() != l.ID.String() {
			t.Fatalf("listing = %v", got)
		}
	})

	t.Run("vacancy", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		l := newVacancy(anna.ID, "Designer")
		e.listings.EXPECT().Create(mock.Anything, listings.CreateParams{
			AuthorID: anna.ID, Kind: listings.KindVacancy, Title: l.Title, Compensation: compensation,
		}).Return(l, nil).Once()
		e.expectUsers(1, anna)
		e.apps.EXPECT().StatsForListings(mock.Anything, []uuid.UUID{l.ID}).Return(noStats(), nil).Once()

		got, err := e.bff.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{
			Kind: hbcv1.ListingKind_LISTING_KIND_VACANCY, Title: l.Title, Compensation: compensation,
		})
		wantOK(t, err)
		require.Equal(t, hbcv1.ListingKind_LISTING_KIND_VACANCY, got.GetKind())
		require.Equal(t, compensation, got.GetCompensation())
	})

	// Shape errors are answered without calling ports (no expectations).
	for _, tt := range []struct {
		name string
		req  *hbcv1.V1CreateListingRequest
		msg  string
	}{
		{"no kind", &hbcv1.V1CreateListingRequest{Title: "No kind"}, msgKindRequired},
		{"unknown kind", &hbcv1.V1CreateListingRequest{Kind: 7, Title: "Bad kind"}, msgKindRequired},
		{"unknown stage", &hbcv1.V1CreateListingRequest{Kind: hbcv1.ListingKind_LISTING_KIND_STARTUP, Stage: 9}, msgBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := newEnv(t).bff.V1CreateListing(ctx, tt.req)
			wantStatus(t, err, codes.InvalidArgument, tt.msg)
		})
	}

	// Content rules come from the context as is.
	for _, tt := range []struct {
		name string
		err  error
		code codes.Code
		msg  string
	}{
		{"stage required", apperr.Validation("stage", cmsgStageRequired), codes.InvalidArgument, cmsgStageRequired},
		{"short title", apperr.Validation("title", cmsgTitle), codes.InvalidArgument, cmsgTitle},
		{caseInternal, errors.New(errDB), codes.Internal, msgInternal},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.listings.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, tt.err).Once()

			_, err := e.bff.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{Kind: hbcv1.ListingKind_LISTING_KIND_STARTUP, Title: "ab"})
			wantStatus(t, err, tt.code, tt.msg)
		})
	}
}

func TestV1GetListing(t *testing.T) {
	t.Parallel()
	anna, boris := newUser(nameAnna), newUser(nameBoris)
	fin := newStartup(anna.ID, titleFinTrack)
	req := &hbcv1.V1GetListingRequest{Id: fin.ID.String()}

	// Author: stats as the context counts them, no myApplication.
	t.Run("author", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().Get(mock.Anything, fin.ID).Return(fin, nil).Once()
		e.expectUsers(1, anna)
		e.apps.EXPECT().StatsForListings(mock.Anything, []uuid.UUID{fin.ID}).
			Return(map[uuid.UUID]applications.Stats{fin.ID: {Pending: 1, Accepted: 1, Declined: 1, Total: 3}}, nil).Once()

		l, err := e.bff.V1GetListing(as(anna.ID), req)
		wantOK(t, err)
		s := l.GetStats()
		if !l.GetIsMine() || l.GetMyApplication() != nil || s.GetPending() != 1 || s.GetAccepted() != 1 || s.GetDeclined() != 1 || s.GetTotal() != 3 {
			t.Fatalf("author view = %v", l)
		}
	})

	// Applicant: myApplication, no stats.
	t.Run("applicant", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().Get(mock.Anything, fin.ID).Return(fin, nil).Once()
		e.expectUsers(1, anna, boris)
		app := withStatus(newApp(fin, boris.ID), applications.StatusAccepted)
		e.apps.EXPECT().GetMineForListings(mock.Anything, boris.ID, []uuid.UUID{fin.ID}).
			Return(map[uuid.UUID]*applications.Application{fin.ID: app}, nil).Once()

		l, err := e.bff.V1GetListing(as(boris.ID), req)
		wantOK(t, err)
		if l.GetIsMine() || l.GetStats() != nil || l.GetMyApplication().GetStatus() != hbcv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED ||
			l.GetMyApplication().GetDecidedAt() == nil || l.GetMyApplication().GetApplicant().GetUsername() != nameBoris {
			t.Fatalf("applicant view = %v", l)
		}
	})

	// Closed listings are visible too; a withdrawn applicant gets no myApplication.
	t.Run("closed", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().Get(mock.Anything, fin.ID).Return(closed(fin), nil).Once()
		e.expectUsers(1, anna, boris)
		e.apps.EXPECT().GetMineForListings(mock.Anything, boris.ID, []uuid.UUID{fin.ID}).
			Return(map[uuid.UUID]*applications.Application{}, nil).Once()

		l, err := e.bff.V1GetListing(as(boris.ID), req)
		wantOK(t, err)
		if l.GetStatus() != hbcv1.ListingStatus_LISTING_STATUS_CLOSED || l.GetClosedAt() == nil || l.GetMyApplication() != nil {
			t.Fatalf("closed listing = %v", l)
		}
	})

	t.Run("bad id", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t) // no expectations: ports are not called
		_, err := e.bff.V1GetListing(as(boris.ID), &hbcv1.V1GetListingRequest{Id: "nope"})
		wantStatus(t, err, codes.NotFound, msgListingNotFound)
	})

	t.Run("unknown id", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().Get(mock.Anything, boris.ID).Return(nil, errOf(apperr.KindNotFound, msgListingNotFound)).Once()

		_, err := e.bff.V1GetListing(as(boris.ID), &hbcv1.V1GetListingRequest{Id: boris.ID.String()})
		wantStatus(t, err, codes.NotFound, msgListingNotFound)
	})
}

// TestComposeListingErrors: a failing batch of the composition is Internal.
func TestComposeListingErrors(t *testing.T) {
	t.Parallel()
	anna, boris := newUser(nameAnna), newUser(nameBoris)
	fin := newStartup(anna.ID, titleFinTrack)
	boom := errors.New(errDB)

	for _, tt := range []struct {
		name   string
		me     uuid.UUID
		expect func(e *env)
	}{
		{"users", boris.ID, func(e *env) {
			e.users.EXPECT().GetMany(mock.Anything, mock.Anything).Return(nil, boom).Once()
		}},
		{"my applications", boris.ID, func(e *env) {
			e.expectUsers(1, anna, boris)
			e.apps.EXPECT().GetMineForListings(mock.Anything, boris.ID, mock.Anything).Return(nil, boom).Once()
		}},
		{"stats", anna.ID, func(e *env) {
			e.expectUsers(1, anna)
			e.apps.EXPECT().StatsForListings(mock.Anything, mock.Anything).Return(nil, boom).Once()
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.listings.EXPECT().Get(mock.Anything, fin.ID).Return(fin, nil).Once()
			tt.expect(e)

			_, err := e.bff.V1GetListing(as(tt.me), &hbcv1.V1GetListingRequest{Id: fin.ID.String()})
			wantStatus(t, err, codes.Internal, msgInternal)
		})
	}
}

func TestV1UpdateListing(t *testing.T) {
	t.Parallel()
	anna, boris := newUser(nameAnna), newUser(nameBoris)
	fin := newStartup(anna.ID, titleFinTrack)
	req := &hbcv1.V1UpdateListingRequest{
		Id: fin.ID.String(), Title: "FinTrack 2", Description: "Новое описание", LookingFor: "Дизайнер",
		Tags: []string{"Design"}, Stage: hbcv1.StartupStage_STARTUP_STAGE_LAUNCHED, Compensation: compensation,
	}
	params := listings.UpdateParams{
		Title: req.GetTitle(), Description: req.GetDescription(), LookingFor: req.GetLookingFor(),
		Tags: req.GetTags(), Stage: listings.StageLaunched, Compensation: compensation,
	}

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		updated := *fin
		updated.Title, updated.Description, updated.LookingFor = params.Title, params.Description, params.LookingFor
		updated.Tags, updated.Stage = params.Tags, params.Stage
		e.listings.EXPECT().Update(mock.Anything, fin.ID, anna.ID, params).Return(&updated, nil).Once()
		e.expectUsers(1, anna)
		e.apps.EXPECT().StatsForListings(mock.Anything, []uuid.UUID{fin.ID}).Return(noStats(), nil).Once()

		l, err := e.bff.V1UpdateListing(as(anna.ID), req)
		wantOK(t, err)
		if l.GetTitle() != params.Title || l.GetLookingFor() != params.LookingFor || l.GetStage() != hbcv1.StartupStage_STARTUP_STAGE_LAUNCHED ||
			l.GetKind() != hbcv1.ListingKind_LISTING_KIND_STARTUP || !l.GetIsMine() || l.GetStats() == nil {
			t.Fatalf("listing = %v", l)
		}
	})

	for _, tt := range []struct {
		name string
		err  error
		code codes.Code
		msg  string
	}{
		{"not the author", errOf(apperr.KindForbidden, cmsgForbiddenEdit), codes.PermissionDenied, cmsgForbiddenEdit},
		{"closed", errOf(apperr.KindPrecondition, cmsgListingClosed), codes.FailedPrecondition, cmsgListingClosed},
		{"unknown listing", errOf(apperr.KindNotFound, msgListingNotFound), codes.NotFound, msgListingNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.listings.EXPECT().Update(mock.Anything, fin.ID, boris.ID, params).Return(nil, tt.err).Once()

			_, err := e.bff.V1UpdateListing(as(boris.ID), req)
			wantStatus(t, err, tt.code, tt.msg)
		})
	}

	// Shape errors: no port calls (no expectations).
	t.Run("bad id", func(t *testing.T) {
		t.Parallel()
		_, err := newEnv(t).bff.V1UpdateListing(as(anna.ID), &hbcv1.V1UpdateListingRequest{Id: "x"})
		wantStatus(t, err, codes.NotFound, msgListingNotFound)
	})
	t.Run("bad stage", func(t *testing.T) {
		t.Parallel()
		_, err := newEnv(t).bff.V1UpdateListing(as(anna.ID), &hbcv1.V1UpdateListingRequest{Id: fin.ID.String(), Stage: 42})
		wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	})
}

func TestV1CloseListing(t *testing.T) {
	t.Parallel()
	anna, boris := newUser(nameAnna), newUser(nameBoris)
	fin := newStartup(anna.ID, titleFinTrack)
	req := &hbcv1.V1CloseListingRequest{Id: fin.ID.String()}

	t.Run("not the author", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().Close(mock.Anything, fin.ID, boris.ID).Return(nil, errOf(apperr.KindForbidden, cmsgForbiddenEdit)).Once()

		_, err := e.bff.V1CloseListing(as(boris.ID), req)
		wantStatus(t, err, codes.PermissionDenied, cmsgForbiddenEdit)
	})

	// Idempotent: the context returns the same closed listing, the BFF passes it.
	t.Run("idempotent", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		c := closed(fin)
		e.listings.EXPECT().Close(mock.Anything, fin.ID, anna.ID).Return(c, nil).Twice()
		e.expectUsers(2, anna)
		e.apps.EXPECT().StatsForListings(mock.Anything, []uuid.UUID{fin.ID}).Return(noStats(), nil).Twice()

		for range 2 {
			l, err := e.bff.V1CloseListing(as(anna.ID), req)
			wantOK(t, err)
			require.Equal(t, hbcv1.ListingStatus_LISTING_STATUS_CLOSED, l.GetStatus())
			require.Equal(t, *c.ClosedAt, l.GetClosedAt().AsTime())
		}
	})

	t.Run("bad id", func(t *testing.T) {
		t.Parallel()
		_, err := newEnv(t).bff.V1CloseListing(as(anna.ID), &hbcv1.V1CloseListingRequest{Id: "x"})
		wantStatus(t, err, codes.NotFound, msgListingNotFound)
	})
}

func TestV1ListMyListings(t *testing.T) {
	t.Parallel()
	anna := newUser(nameAnna)
	a1, a2 := newStartup(anna.ID, "First"), closed(newVacancy(anna.ID, "Second"))
	ctx := as(anna.ID)

	// Stats for every own listing (zero when the context has no key); port order kept.
	t.Run("all", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().ListByAuthor(mock.Anything, anna.ID, listings.Status("")).Return([]*listings.Listing{a2, a1}, nil).Once()
		e.expectUsers(1, anna)
		e.apps.EXPECT().StatsForListings(mock.Anything, []uuid.UUID{a2.ID, a1.ID}).
			Return(map[uuid.UUID]applications.Stats{a1.ID: {Pending: 1, Total: 1}}, nil).Once()

		resp, err := e.bff.V1ListMyListings(ctx, &hbcv1.V1ListMyListingsRequest{})
		wantOK(t, err)
		items := resp.GetItems()
		require.Equal(t, []string{a2.ID.String(), a1.ID.String()}, listingIDs(items))
		for _, l := range items {
			if !l.GetIsMine() || l.GetStats() == nil || l.GetMyApplication() != nil {
				t.Fatalf("own listing = %v", l)
			}
		}
		require.Zero(t, items[0].GetStats().GetTotal())
		require.EqualValues(t, 1, items[1].GetStats().GetPending())
		require.EqualValues(t, 1, items[1].GetStats().GetTotal())
		require.Equal(t, hbcv1.ListingStatus_LISTING_STATUS_CLOSED, items[0].GetStatus())
	})

	t.Run("status filter", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().ListByAuthor(mock.Anything, anna.ID, listings.StatusActive).Return([]*listings.Listing{a1}, nil).Once()
		e.expectUsers(1, anna)
		e.apps.EXPECT().StatsForListings(mock.Anything, []uuid.UUID{a1.ID}).Return(noStats(), nil).Once()

		resp, err := e.bff.V1ListMyListings(ctx, &hbcv1.V1ListMyListingsRequest{Status: hbcv1.ListingStatus_LISTING_STATUS_ACTIVE})
		wantOK(t, err)
		require.Equal(t, []string{a1.ID.String()}, listingIDs(resp.GetItems()))
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.listings.EXPECT().ListByAuthor(mock.Anything, anna.ID, listings.StatusClosed).Return(nil, nil).Once()

		resp, err := e.bff.V1ListMyListings(ctx, &hbcv1.V1ListMyListingsRequest{Status: hbcv1.ListingStatus_LISTING_STATUS_CLOSED})
		wantOK(t, err)
		require.NotNil(t, resp.GetItems())
		require.Empty(t, resp.GetItems())
	})

	t.Run("bad status", func(t *testing.T) {
		t.Parallel()
		_, err := newEnv(t).bff.V1ListMyListings(ctx, &hbcv1.V1ListMyListingsRequest{Status: 5})
		wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	})

	for _, tt := range []struct {
		name   string
		expect func(e *env)
	}{
		{"list fails", func(e *env) {
			e.listings.EXPECT().ListByAuthor(mock.Anything, anna.ID, listings.Status("")).Return(nil, errors.New(errDB)).Once()
		}},
		{"compose fails", func(e *env) {
			e.listings.EXPECT().ListByAuthor(mock.Anything, anna.ID, listings.Status("")).Return([]*listings.Listing{a1}, nil).Once()
			e.users.EXPECT().GetMany(mock.Anything, mock.Anything).Return(nil, errors.New(errDB)).Once()
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			tt.expect(e)
			_, err := e.bff.V1ListMyListings(ctx, &hbcv1.V1ListMyListingsRequest{})
			wantStatus(t, err, codes.Internal, msgInternal)
		})
	}
}
