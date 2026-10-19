package bff

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

func TestV1CreateApplication(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris := e.user(t, nameAnna), e.user(t, nameBoris)
	fin := e.startup(t, anna.ID, titleFinTrack)
	req := &hbcv1.V1CreateApplicationRequest{ListingId: fin.ID.String(), Message: "Хочу в команду"}

	a, err := e.bff.V1CreateApplication(as(boris.ID), req)
	wantOK(t, err)
	if a.GetStatus() != hbcv1.ApplicationStatus_APPLICATION_STATUS_PENDING || a.GetMessage() != "Хочу в команду" ||
		a.GetApplicant().GetUsername() != nameBoris || a.GetDecidedAt() != nil || a.GetCreatedAt() == nil || a.GetUpdatedAt() == nil {
		t.Fatalf("application = %v", a)
	}
	if s := a.GetListing(); s.GetId() != fin.ID.String() || s.GetTitle() != titleFinTrack || s.GetKind() != hbcv1.ListingKind_LISTING_KIND_STARTUP ||
		s.GetStatus() != hbcv1.ListingStatus_LISTING_STATUS_ACTIVE || s.GetAuthor().GetUsername() != nameAnna {
		t.Fatalf("listing summary = %v", s)
	}

	_, err = e.bff.V1CreateApplication(as(boris.ID), req)
	wantStatus(t, err, codes.AlreadyExists, fmsgAlreadyApplied)

	_, err = e.bff.V1CreateApplication(as(anna.ID), req)
	wantStatus(t, err, codes.FailedPrecondition, fmsgOwnListing)

	vera := e.user(t, "vera")
	_, err = e.bff.V1CreateApplication(as(vera.ID), &hbcv1.V1CreateApplicationRequest{ListingId: fin.ID.String()})
	wantStatus(t, err, codes.InvalidArgument, fmsgMessage)

	// After withdraw a new application can be created.
	_, err = e.bff.V1WithdrawApplication(as(boris.ID), &hbcv1.V1ApplicationIDRequest{Id: a.GetId()})
	wantOK(t, err)
	again, err := e.bff.V1CreateApplication(as(boris.ID), req)
	wantOK(t, err)
	if again.GetId() == a.GetId() {
		t.Fatal("re-application must be a new record")
	}

	e.closeListing(t, fin)
	_, err = e.bff.V1CreateApplication(as(vera.ID), req)
	wantStatus(t, err, codes.FailedPrecondition, fmsgListingClosed)

	e.calls.reset()
	_, err = e.bff.V1CreateApplication(as(vera.ID), &hbcv1.V1CreateApplicationRequest{ListingId: badID, Message: "x"})
	wantStatus(t, err, codes.NotFound, msgListingNotFound)
	e.noCalls(t)
	_, err = e.bff.V1CreateApplication(as(vera.ID), &hbcv1.V1CreateApplicationRequest{ListingId: vera.ID.String(), Message: "x"})
	wantStatus(t, err, codes.NotFound, msgListingNotFound)
}

func TestV1ListListingApplications(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris, vera := e.user(t, nameAnna), e.user(t, nameBoris), e.user(t, "vera")
	fin := e.startup(t, anna.ID, titleFinTrack)
	first := e.apply(t, fin, boris.ID)
	second := e.apply(t, fin, vera.ID)
	e.setStatus(t, second, applications.StatusDeclined)
	ctx := as(anna.ID)

	resp, err := e.bff.V1ListListingApplications(ctx, &hbcv1.V1ListListingApplicationsRequest{ListingId: fin.ID.String()})
	wantOK(t, err)
	items := resp.GetItems()
	if len(items) != 2 || items[0].GetId() != first.ID.String() || items[1].GetId() != second.ID.String() { // pending first
		t.Fatalf("items = %v", items)
	}
	if items[0].GetApplicant().GetUsername() != nameBoris || items[1].GetApplicant().GetUsername() != "vera" ||
		items[0].GetListing().GetAuthor().GetUsername() != nameAnna {
		t.Fatalf("composition = %v", items)
	}

	resp, err = e.bff.V1ListListingApplications(ctx, &hbcv1.V1ListListingApplicationsRequest{
		ListingId: fin.ID.String(), Status: hbcv1.ApplicationStatus_APPLICATION_STATUS_DECLINED,
	})
	wantOK(t, err)
	if len(resp.GetItems()) != 1 || resp.GetItems()[0].GetDecidedAt() == nil {
		t.Fatalf("declined = %v", resp.GetItems())
	}

	_, err = e.bff.V1ListListingApplications(as(boris.ID), &hbcv1.V1ListListingApplicationsRequest{ListingId: fin.ID.String()})
	wantStatus(t, err, codes.PermissionDenied, fmsgForbiddenListAp)

	e.calls.reset()
	_, err = e.bff.V1ListListingApplications(ctx, &hbcv1.V1ListListingApplicationsRequest{ListingId: badID})
	wantStatus(t, err, codes.NotFound, msgListingNotFound)
	_, err = e.bff.V1ListListingApplications(ctx, &hbcv1.V1ListListingApplicationsRequest{ListingId: fin.ID.String(), Status: 10})
	wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	e.noCalls(t)
}

func TestV1AcceptDeclineApplication(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris, vera := e.user(t, nameAnna), e.user(t, nameBoris), e.user(t, "vera")
	fin := e.startup(t, anna.ID, titleFinTrack)
	toAccept := &hbcv1.V1ApplicationIDRequest{Id: e.apply(t, fin, boris.ID).ID.String()}
	toDecline := &hbcv1.V1ApplicationIDRequest{Id: e.apply(t, fin, vera.ID).ID.String()}
	ctx := as(anna.ID)

	for range 2 { // idempotent
		a, err := e.bff.V1AcceptApplication(ctx, toAccept)
		wantOK(t, err)
		if a.GetStatus() != hbcv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED || a.GetDecidedAt() == nil || a.GetApplicant().GetUsername() != nameBoris {
			t.Fatalf("accepted = %v", a)
		}
	}
	_, err := e.bff.V1DeclineApplication(ctx, toAccept)
	wantStatus(t, err, codes.FailedPrecondition, fmsgDecided)

	for range 2 {
		a, err := e.bff.V1DeclineApplication(ctx, toDecline)
		wantOK(t, err)
		if a.GetStatus() != hbcv1.ApplicationStatus_APPLICATION_STATUS_DECLINED || a.GetDecidedAt() == nil {
			t.Fatalf("declined = %v", a)
		}
	}
	_, err = e.bff.V1AcceptApplication(ctx, toDecline)
	wantStatus(t, err, codes.FailedPrecondition, fmsgDecided)

	// The applicant is the other side of the application: 403.
	_, err = e.bff.V1AcceptApplication(as(boris.ID), toAccept)
	wantStatus(t, err, codes.PermissionDenied, fmsgNoRights)
	_, err = e.bff.V1DeclineApplication(as(boris.ID), toAccept)
	wantStatus(t, err, codes.PermissionDenied, fmsgNoRights)
	// A stranger: the application is not disclosed, 404.
	_, err = e.bff.V1AcceptApplication(as(vera.ID), toAccept)
	wantStatus(t, err, codes.NotFound, msgApplicationNotFound)
	_, err = e.bff.V1DeclineApplication(as(vera.ID), toAccept)
	wantStatus(t, err, codes.NotFound, msgApplicationNotFound)

	e.calls.reset()
	_, err = e.bff.V1AcceptApplication(ctx, &hbcv1.V1ApplicationIDRequest{Id: badID})
	wantStatus(t, err, codes.NotFound, msgApplicationNotFound)
	_, err = e.bff.V1DeclineApplication(ctx, &hbcv1.V1ApplicationIDRequest{})
	wantStatus(t, err, codes.NotFound, msgApplicationNotFound)
	e.noCalls(t)
}

func TestV1WithdrawApplication(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris, vera := e.user(t, nameAnna), e.user(t, nameBoris), e.user(t, "vera")
	fin := e.startup(t, anna.ID, titleFinTrack)
	pending := &hbcv1.V1ApplicationIDRequest{Id: e.apply(t, fin, boris.ID).ID.String()}
	accepted := e.apply(t, fin, vera.ID)
	e.setStatus(t, accepted, applications.StatusAccepted)

	_, err := e.bff.V1WithdrawApplication(as(anna.ID), pending) // the listing author: 403
	wantStatus(t, err, codes.PermissionDenied, fmsgNoRights)
	_, err = e.bff.V1WithdrawApplication(as(vera.ID), pending) // a stranger: 404
	wantStatus(t, err, codes.NotFound, msgApplicationNotFound)

	for range 2 { // idempotent
		a, err := e.bff.V1WithdrawApplication(as(boris.ID), pending)
		wantOK(t, err)
		if a.GetStatus() != hbcv1.ApplicationStatus_APPLICATION_STATUS_WITHDRAWN || a.GetDecidedAt() != nil {
			t.Fatalf("withdrawn = %v", a)
		}
	}

	_, err = e.bff.V1WithdrawApplication(as(vera.ID), &hbcv1.V1ApplicationIDRequest{Id: accepted.ID.String()})
	wantStatus(t, err, codes.FailedPrecondition, fmsgDecided)

	_, err = e.bff.V1WithdrawApplication(as(vera.ID), &hbcv1.V1ApplicationIDRequest{Id: badID})
	wantStatus(t, err, codes.NotFound, msgApplicationNotFound)
}

func TestV1ListSentAndReceivedApplications(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris, vera := e.user(t, nameAnna), e.user(t, nameBoris), e.user(t, "vera")
	fin := e.startup(t, anna.ID, titleFinTrack)
	job := e.vacancy(t, anna.ID, "Designer")
	other := e.startup(t, vera.ID, "Other")
	a1 := e.apply(t, fin, boris.ID)
	a2 := e.apply(t, job, boris.ID)
	a3 := e.apply(t, other, boris.ID)
	a4 := e.apply(t, job, vera.ID)
	e.setStatus(t, a1, applications.StatusAccepted)

	sent, err := e.bff.V1ListSentApplications(as(boris.ID), &hbcv1.V1ListSentApplicationsRequest{})
	wantOK(t, err)
	if ids := appIDs(sent.GetItems()); ids != joinIDs(a3.ID, a2.ID, a1.ID) { // created_at DESC
		t.Fatalf("sent = %s", ids)
	}
	for _, a := range sent.GetItems() {
		if a.GetApplicant().GetUsername() != nameBoris || a.GetListing().GetAuthor() == nil {
			t.Fatalf("composition = %v", a)
		}
	}
	sent, err = e.bff.V1ListSentApplications(as(boris.ID), &hbcv1.V1ListSentApplicationsRequest{Status: hbcv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED})
	wantOK(t, err)
	if ids := appIDs(sent.GetItems()); ids != joinIDs(a1.ID) {
		t.Fatalf("sent accepted = %s", ids)
	}

	received, err := e.bff.V1ListReceivedApplications(as(anna.ID), &hbcv1.V1ListReceivedApplicationsRequest{})
	wantOK(t, err)
	if ids := appIDs(received.GetItems()); ids != joinIDs(a4.ID, a2.ID, a1.ID) { // pending first, then created_at DESC
		t.Fatalf("received = %s", ids)
	}
	received, err = e.bff.V1ListReceivedApplications(as(anna.ID), &hbcv1.V1ListReceivedApplicationsRequest{Status: hbcv1.ApplicationStatus_APPLICATION_STATUS_PENDING})
	wantOK(t, err)
	if ids := appIDs(received.GetItems()); ids != joinIDs(a4.ID, a2.ID) {
		t.Fatalf("received pending = %s", ids)
	}

	empty, err := e.bff.V1ListReceivedApplications(as(boris.ID), &hbcv1.V1ListReceivedApplicationsRequest{})
	wantOK(t, err)
	if empty.GetItems() == nil || len(empty.GetItems()) != 0 {
		t.Fatalf("empty = %v", empty.GetItems())
	}

	_, err = e.bff.V1ListSentApplications(as(boris.ID), &hbcv1.V1ListSentApplicationsRequest{Status: 99})
	wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
	_, err = e.bff.V1ListReceivedApplications(as(boris.ID), &hbcv1.V1ListReceivedApplicationsRequest{Status: 99})
	wantStatus(t, err, codes.InvalidArgument, msgBadRequest)
}

// TestApplicationListsBatches: composing lists of applications takes two port calls
// (Listings.GetMany, Users.GetMany) for any number of items.
func TestApplicationListsBatches(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna := e.user(t, nameAnna)
	for i := range 10 {
		l := e.startup(t, anna.ID, fmt.Sprintf("Listing %d", i))
		e.apply(t, l, e.user(t, fmt.Sprintf("user%d", i)).ID)
	}
	e.calls.reset()

	resp, err := e.bff.V1ListReceivedApplications(as(anna.ID), &hbcv1.V1ListReceivedApplicationsRequest{})
	wantOK(t, err)
	if len(resp.GetItems()) != 10 {
		t.Fatalf("items = %d", len(resp.GetItems()))
	}
	want := map[string]int{"Applications.ListReceived": 1, "Listings.GetMany": 1, "Users.GetMany": 1}
	if got := e.calls.snapshot(); !maps.Equal(got, want) {
		t.Fatalf("port calls = %v, want %v", got, want)
	}
}

func TestApplicationsInternalError(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	anna, boris := e.user(t, nameAnna), e.user(t, nameBoris)
	fin := e.startup(t, anna.ID, titleFinTrack)
	e.apply(t, fin, boris.ID)

	e.listings.err = errors.New("pgx: broken pipe") // composition failure
	_, err := e.bff.V1ListSentApplications(as(boris.ID), &hbcv1.V1ListSentApplicationsRequest{})
	wantStatus(t, err, codes.Internal, msgInternal)
}

func appIDs(items []*hbcv1.Application) string {
	ids := make([]string, 0, len(items))
	for _, a := range items {
		ids = append(ids, a.GetId())
	}
	return strings.Join(ids, " ")
}

func joinIDs(ids ...uuid.UUID) string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return strings.Join(out, " ")
}
