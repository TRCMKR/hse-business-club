package bff

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

func TestV1ListListingsComposition(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris := e.user(t, nameAnna), e.user(t, nameBoris)
	fin := e.startup(t, anna.ID, titleFinTrack)
	closed := e.vacancy(t, anna.ID, "Closed vacancy")
	e.closeListing(t, closed)
	own := e.vacancy(t, boris.ID, "Boris vacancy")
	app := e.apply(t, fin, boris.ID)

	resp, err := e.bff.V1ListListings(as(boris.ID), &hbcv1.V1ListListingsRequest{})
	wantOK(t, err)
	if resp.GetTotal() != 2 || resp.GetPage() != 1 || resp.GetLimit() != 20 || len(resp.GetItems()) != 2 {
		t.Fatalf("resp = total %d page %d limit %d items %d", resp.GetTotal(), resp.GetPage(), resp.GetLimit(), len(resp.GetItems()))
	}
	mine, foreign := resp.GetItems()[0], resp.GetItems()[1] // newest first

	if mine.GetId() != own.ID.String() || !mine.GetIsMine() || mine.GetMyApplication() != nil {
		t.Fatalf("own listing = %v", mine)
	}
	if s := mine.GetStats(); s == nil || s.GetTotal() != 0 {
		t.Fatalf("own listing stats = %v, want zero stats", s)
	}
	if mine.GetAuthor().GetUsername() != nameBoris || mine.GetKind() != hbcv1.ListingKind_LISTING_KIND_VACANCY ||
		mine.GetCompensation() != "Доля" || mine.GetStage() != hbcv1.StartupStage_STARTUP_STAGE_UNSPECIFIED {
		t.Fatalf("own listing = %v", mine)
	}

	if foreign.GetId() != fin.ID.String() || foreign.GetIsMine() || foreign.GetStats() != nil {
		t.Fatalf("foreign listing = %v", foreign)
	}
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

	// A withdrawn application is not "mine" any more.
	e.setStatus(t, app, applications.StatusWithdrawn)
	resp, err = e.bff.V1ListListings(as(boris.ID), &hbcv1.V1ListListingsRequest{Kind: hbcv1.ListingKind_LISTING_KIND_STARTUP})
	wantOK(t, err)
	if len(resp.GetItems()) != 1 || resp.GetItems()[0].GetMyApplication() != nil {
		t.Fatalf("startup feed after withdraw = %v", resp.GetItems())
	}
}

func TestV1ListListingsParams(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)
	for _, title := range []string{"Alpha", "Beta", "Gamma"} {
		e.startup(t, anna.ID, title)
	}
	ctx := as(anna.ID)

	resp, err := e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{Query: "beta"})
	wantOK(t, err)
	if resp.GetTotal() != 1 || resp.GetItems()[0].GetTitle() != "Beta" {
		t.Fatalf("query result = %v", resp)
	}

	resp, err = e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{Page: 2, Limit: 2})
	wantOK(t, err)
	if resp.GetTotal() != 3 || resp.GetPage() != 2 || resp.GetLimit() != 2 || len(resp.GetItems()) != 1 || resp.GetItems()[0].GetTitle() != "Alpha" {
		t.Fatalf("page 2 = %v", resp)
	}

	_, err = e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{Limit: 51})
	wantStatus(t, err, codes.InvalidArgument, fmsgLimit)

	e.calls.reset()
	_, err = e.bff.V1ListListings(ctx, &hbcv1.V1ListListingsRequest{Kind: 99})
	wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	e.noCalls(t)
}

// TestV1ListListingsBatches: the number of port calls does not depend on the page size.
func TestV1ListListingsBatches(t *testing.T) {
	t.Parallel()

	countCalls := func(n int) map[string]int {
		e := newEnv(t)
		me := e.user(t, "myself")
		for i := range n {
			author := e.user(t, fmt.Sprintf("author%d", i))
			l := e.startup(t, author.ID, "Foreign listing")
			if i%2 == 0 {
				e.apply(t, l, me.ID)
			}
			e.startup(t, me.ID, "My listing")
		}
		e.calls.reset()
		resp, err := e.bff.V1ListListings(as(me.ID), &hbcv1.V1ListListingsRequest{Limit: 50})
		wantOK(t, err)
		if len(resp.GetItems()) != min(2*n, 50) {
			t.Fatalf("items = %d", len(resp.GetItems()))
		}
		return e.calls.snapshot()
	}

	one, many := countCalls(1), countCalls(25)
	want := map[string]int{
		"Listings.ListActive":             1,
		"Users.GetMany":                   1,
		"Applications.GetMineForListings": 1,
		"Applications.StatsForListings":   1,
	}
	if !maps.Equal(one, want) || !maps.Equal(many, want) {
		t.Fatalf("port calls: N=1 %v, N=25 %v, want %v", one, many, want)
	}
}

func TestV1CreateListing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)
	ctx := as(anna.ID)

	l, err := e.bff.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{
		Kind: hbcv1.ListingKind_LISTING_KIND_STARTUP, Title: titleFinTrack, Description: "Учёт финансов",
		LookingFor: lookingForCTO, Tags: []string{"Go", lookingForCTO}, Stage: hbcv1.StartupStage_STARTUP_STAGE_IDEA,
	})
	wantOK(t, err)
	if !l.GetIsMine() || l.GetAuthor().GetId() != anna.ID.String() || l.GetStats() == nil || l.GetMyApplication() != nil ||
		l.GetStage() != hbcv1.StartupStage_STARTUP_STAGE_IDEA || l.GetStatus() != hbcv1.ListingStatus_LISTING_STATUS_ACTIVE ||
		l.GetClosedAt() != nil || l.GetTitle() != titleFinTrack {
		t.Fatalf("listing = %v", l)
	}

	e.calls.reset()
	_, err = e.bff.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{Title: "No kind"})
	wantStatus(t, err, codes.InvalidArgument, msgKindRequired)
	_, err = e.bff.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{Kind: 7, Title: "Bad kind"})
	wantStatus(t, err, codes.InvalidArgument, msgKindRequired)
	_, err = e.bff.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{Kind: hbcv1.ListingKind_LISTING_KIND_STARTUP, Stage: 9})
	wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	e.noCalls(t)

	// Content rules come from the context as is.
	_, err = e.bff.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{Kind: hbcv1.ListingKind_LISTING_KIND_STARTUP, Title: titleFinTrack})
	wantStatus(t, err, codes.InvalidArgument, fmsgStageRequired)
	_, err = e.bff.V1CreateListing(ctx, &hbcv1.V1CreateListingRequest{Kind: hbcv1.ListingKind_LISTING_KIND_VACANCY, Title: "ab"})
	wantStatus(t, err, codes.InvalidArgument, fmsgTitle)
}

func TestV1GetListing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris, vera, gleb, dina := e.user(t, nameAnna), e.user(t, nameBoris), e.user(t, "vera"), e.user(t, "gleb"), e.user(t, "dina")
	fin := e.startup(t, anna.ID, titleFinTrack)
	e.setStatus(t, e.apply(t, fin, boris.ID), applications.StatusAccepted)
	e.setStatus(t, e.apply(t, fin, vera.ID), applications.StatusDeclined)
	e.setStatus(t, e.apply(t, fin, gleb.ID), applications.StatusWithdrawn)
	e.apply(t, fin, dina.ID)

	// Author: stats without withdrawn, no myApplication.
	l, err := e.bff.V1GetListing(as(anna.ID), &hbcv1.V1GetListingRequest{Id: fin.ID.String()})
	wantOK(t, err)
	s := l.GetStats()
	if !l.GetIsMine() || l.GetMyApplication() != nil || s.GetPending() != 1 || s.GetAccepted() != 1 || s.GetDeclined() != 1 || s.GetTotal() != 3 {
		t.Fatalf("author view = %v", l)
	}

	// Applicant: myApplication, no stats.
	l, err = e.bff.V1GetListing(as(boris.ID), &hbcv1.V1GetListingRequest{Id: fin.ID.String()})
	wantOK(t, err)
	if l.GetIsMine() || l.GetStats() != nil || l.GetMyApplication().GetStatus() != hbcv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED ||
		l.GetMyApplication().GetDecidedAt() == nil {
		t.Fatalf("applicant view = %v", l)
	}

	// Withdrawn applicant: no myApplication.
	l, err = e.bff.V1GetListing(as(gleb.ID), &hbcv1.V1GetListingRequest{Id: fin.ID.String()})
	wantOK(t, err)
	if l.GetMyApplication() != nil {
		t.Fatalf("withdrawn applicant view = %v", l)
	}

	// Closed listings are visible too.
	e.closeListing(t, fin)
	l, err = e.bff.V1GetListing(as(boris.ID), &hbcv1.V1GetListingRequest{Id: fin.ID.String()})
	wantOK(t, err)
	if l.GetStatus() != hbcv1.ListingStatus_LISTING_STATUS_CLOSED || l.GetClosedAt() == nil {
		t.Fatalf("closed listing = %v", l)
	}

	e.calls.reset()
	_, err = e.bff.V1GetListing(as(boris.ID), &hbcv1.V1GetListingRequest{Id: "nope"})
	wantStatus(t, err, codes.NotFound, msgListingNotFound)
	e.noCalls(t)
	_, err = e.bff.V1GetListing(as(boris.ID), &hbcv1.V1GetListingRequest{Id: boris.ID.String()})
	wantStatus(t, err, codes.NotFound, msgListingNotFound)
}

func TestV1UpdateListing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris := e.user(t, nameAnna), e.user(t, nameBoris)
	fin := e.startup(t, anna.ID, titleFinTrack)
	req := &hbcv1.V1UpdateListingRequest{
		Id: fin.ID.String(), Title: "FinTrack 2", Description: "Новое описание", LookingFor: "Дизайнер",
		Tags: []string{"Design"}, Stage: hbcv1.StartupStage_STARTUP_STAGE_LAUNCHED,
	}

	l, err := e.bff.V1UpdateListing(as(anna.ID), req)
	wantOK(t, err)
	if l.GetTitle() != "FinTrack 2" || l.GetLookingFor() != "Дизайнер" || l.GetStage() != hbcv1.StartupStage_STARTUP_STAGE_LAUNCHED ||
		l.GetKind() != hbcv1.ListingKind_LISTING_KIND_STARTUP || !l.GetIsMine() || l.GetStats() == nil {
		t.Fatalf("listing = %v", l)
	}

	_, err = e.bff.V1UpdateListing(as(boris.ID), req)
	wantStatus(t, err, codes.PermissionDenied, fmsgForbiddenEdit)

	e.closeListing(t, fin)
	_, err = e.bff.V1UpdateListing(as(anna.ID), req)
	wantStatus(t, err, codes.FailedPrecondition, fmsgListingClosed)

	e.calls.reset()
	_, err = e.bff.V1UpdateListing(as(anna.ID), &hbcv1.V1UpdateListingRequest{Id: "x"})
	wantStatus(t, err, codes.NotFound, msgListingNotFound)
	_, err = e.bff.V1UpdateListing(as(anna.ID), &hbcv1.V1UpdateListingRequest{Id: fin.ID.String(), Stage: 42})
	wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	e.noCalls(t)
}

func TestV1CloseListing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris := e.user(t, nameAnna), e.user(t, nameBoris)
	fin := e.startup(t, anna.ID, titleFinTrack)

	_, err := e.bff.V1CloseListing(as(boris.ID), &hbcv1.V1CloseListingRequest{Id: fin.ID.String()})
	wantStatus(t, err, codes.PermissionDenied, fmsgForbiddenEdit)

	var closedAt string
	for range 2 { // idempotent
		l, err := e.bff.V1CloseListing(as(anna.ID), &hbcv1.V1CloseListingRequest{Id: fin.ID.String()})
		wantOK(t, err)
		if l.GetStatus() != hbcv1.ListingStatus_LISTING_STATUS_CLOSED || l.GetClosedAt() == nil {
			t.Fatalf("listing = %v", l)
		}
		if closedAt != "" && l.GetClosedAt().AsTime().String() != closedAt {
			t.Fatal("repeated close changed closedAt")
		}
		closedAt = l.GetClosedAt().AsTime().String()
	}

	_, err = e.bff.V1CloseListing(as(anna.ID), &hbcv1.V1CloseListingRequest{Id: "x"})
	wantStatus(t, err, codes.NotFound, msgListingNotFound)
}

func TestV1ListMyListings(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris := e.user(t, nameAnna), e.user(t, nameBoris)
	a1 := e.startup(t, anna.ID, "First")
	a2 := e.vacancy(t, anna.ID, "Second")
	e.startup(t, boris.ID, "Foreign")
	e.apply(t, a1, boris.ID)
	e.closeListing(t, a2)

	resp, err := e.bff.V1ListMyListings(as(anna.ID), &hbcv1.V1ListMyListingsRequest{})
	wantOK(t, err)
	items := resp.GetItems()
	if len(items) != 2 || items[0].GetId() != a2.ID.String() || items[1].GetId() != a1.ID.String() {
		t.Fatalf("items = %v", items)
	}
	for _, l := range items {
		if !l.GetIsMine() || l.GetStats() == nil {
			t.Fatalf("listing without stats: %v", l)
		}
	}
	if items[1].GetStats().GetPending() != 1 || items[1].GetStats().GetTotal() != 1 {
		t.Fatalf("stats = %v", items[1].GetStats())
	}

	resp, err = e.bff.V1ListMyListings(as(anna.ID), &hbcv1.V1ListMyListingsRequest{Status: hbcv1.ListingStatus_LISTING_STATUS_ACTIVE})
	wantOK(t, err)
	if len(resp.GetItems()) != 1 || resp.GetItems()[0].GetId() != a1.ID.String() {
		t.Fatalf("active items = %v", resp.GetItems())
	}

	resp, err = e.bff.V1ListMyListings(as(e.user(t, "vera").ID), &hbcv1.V1ListMyListingsRequest{})
	wantOK(t, err)
	if resp.GetItems() == nil || len(resp.GetItems()) != 0 {
		t.Fatalf("empty list = %v", resp.GetItems())
	}

	_, err = e.bff.V1ListMyListings(as(anna.ID), &hbcv1.V1ListMyListingsRequest{Status: 5})
	wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
}
