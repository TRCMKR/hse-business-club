package bff

import (
	"context"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// V1CreateApplication applies the current user to a listing.
func (i *Implementation) V1CreateApplication(ctx context.Context, req *hbcv1.V1CreateApplicationRequest) (*hbcv1.Application, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	listingID, err := parsePathID(req.GetListingId(), msgListingNotFound)
	if err != nil {
		return nil, err
	}
	a, err := i.Applications.Apply(ctx, listingID, me, req.GetMessage())
	return i.applicationResponse(ctx, a, err)
}

// V1ListListingApplications returns applications to a listing (its author only).
func (i *Implementation) V1ListListingApplications(ctx context.Context, req *hbcv1.V1ListListingApplicationsRequest) (*hbcv1.V1ApplicationsResponse, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	listingID, err := parsePathID(req.GetListingId(), msgListingNotFound)
	if err != nil {
		return nil, err
	}
	st, err := appStatusFilter(req.GetStatus())
	if err != nil {
		return nil, err
	}
	apps, err := i.Applications.ListByListing(ctx, listingID, me, st)
	return i.applicationsResponse(ctx, apps, err)
}

// V1AcceptApplication accepts an application (listing author only).
func (i *Implementation) V1AcceptApplication(ctx context.Context, req *hbcv1.V1ApplicationIDRequest) (*hbcv1.Application, error) {
	return i.transition(ctx, req, i.Applications.Accept)
}

// V1DeclineApplication declines an application (listing author only).
func (i *Implementation) V1DeclineApplication(ctx context.Context, req *hbcv1.V1ApplicationIDRequest) (*hbcv1.Application, error) {
	return i.transition(ctx, req, i.Applications.Decline)
}

// V1WithdrawApplication withdraws an application (applicant only).
func (i *Implementation) V1WithdrawApplication(ctx context.Context, req *hbcv1.V1ApplicationIDRequest) (*hbcv1.Application, error) {
	return i.transition(ctx, req, i.Applications.Withdraw)
}

// V1ListSentApplications returns applications sent by the current user.
func (i *Implementation) V1ListSentApplications(ctx context.Context, req *hbcv1.V1ListSentApplicationsRequest) (*hbcv1.V1ApplicationsResponse, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	st, err := appStatusFilter(req.GetStatus())
	if err != nil {
		return nil, err
	}
	apps, err := i.Applications.ListByApplicant(ctx, me, st)
	return i.applicationsResponse(ctx, apps, err)
}

// V1ListReceivedApplications returns applications to all listings of the current user.
func (i *Implementation) V1ListReceivedApplications(ctx context.Context, req *hbcv1.V1ListReceivedApplicationsRequest) (*hbcv1.V1ApplicationsResponse, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	st, err := appStatusFilter(req.GetStatus())
	if err != nil {
		return nil, err
	}
	apps, err := i.Applications.ListReceived(ctx, me, st)
	return i.applicationsResponse(ctx, apps, err)
}

// transitionFunc is Accept, Decline or Withdraw of applications.Service.
type transitionFunc func(ctx context.Context, id, actorID uuid.UUID) (*applications.Application, error)

func (i *Implementation) transition(ctx context.Context, req *hbcv1.V1ApplicationIDRequest, fn transitionFunc) (*hbcv1.Application, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parsePathID(req.GetId(), msgApplicationNotFound)
	if err != nil {
		return nil, err
	}
	a, err := fn(ctx, id, me)
	return i.applicationResponse(ctx, a, err)
}

// applicationResponse composes the result of a port call that returns one application.
func (i *Implementation) applicationResponse(ctx context.Context, a *applications.Application, err error) (*hbcv1.Application, error) {
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	out, err := i.composeApplication(ctx, a)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return out, nil
}

// applicationsResponse composes the result of a port call that returns a list.
func (i *Implementation) applicationsResponse(ctx context.Context, apps []*applications.Application, err error) (*hbcv1.V1ApplicationsResponse, error) {
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	items, err := i.composeApplications(ctx, apps)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return &hbcv1.V1ApplicationsResponse{Items: items}, nil
}
