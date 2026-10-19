package bff

import (
	"context"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/listings"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// V1ListListings returns a page of the feed of active listings. Port calls do not
// depend on the page size: ListActive, then the batches of composeListings.
func (i *Implementation) V1ListListings(ctx context.Context, req *hbcv1.V1ListListingsRequest) (*hbcv1.V1ListListingsResponse, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	kind, err := kindFilter(req.GetKind())
	if err != nil {
		return nil, err
	}
	page, limit := pageParams(req.GetPage(), req.GetLimit())
	p, err := i.Listings.ListActive(ctx, listings.ListFilter{
		Kind:  kind,
		Query: req.GetQuery(),
		Page:  page,
		Limit: limit,
	})
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	items, err := i.composeListings(ctx, me, p.Items)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return &hbcv1.V1ListListingsResponse{
		Items: items,
		Total: int32(p.Total), //nolint:gosec // bounded by the number of listings
		Page:  int32(p.Page),  //nolint:gosec // validated by the context
		Limit: int32(p.Limit), //nolint:gosec // validated by the context
	}, nil
}

// V1CreateListing publishes a startup or a vacancy authored by the current user.
func (i *Implementation) V1CreateListing(ctx context.Context, req *hbcv1.V1CreateListingRequest) (*hbcv1.Listing, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	kind, err := requiredKind(req.GetKind())
	if err != nil {
		return nil, err
	}
	stage, err := stageParam(req.GetStage())
	if err != nil {
		return nil, err
	}
	l, err := i.Listings.Create(ctx, listings.CreateParams{
		AuthorID:     me,
		Kind:         kind,
		Title:        req.GetTitle(),
		Description:  req.GetDescription(),
		LookingFor:   req.GetLookingFor(),
		Tags:         req.GetTags(),
		Stage:        stage,
		Compensation: req.GetCompensation(),
	})
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return i.listingResponse(ctx, me, l)
}

// V1GetListing returns a listing (active or closed).
func (i *Implementation) V1GetListing(ctx context.Context, req *hbcv1.V1GetListingRequest) (*hbcv1.Listing, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parsePathID(req.GetId(), msgListingNotFound)
	if err != nil {
		return nil, err
	}
	l, err := i.Listings.Get(ctx, id)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return i.listingResponse(ctx, me, l)
}

// V1UpdateListing replaces the editable fields of an active listing (author only).
func (i *Implementation) V1UpdateListing(ctx context.Context, req *hbcv1.V1UpdateListingRequest) (*hbcv1.Listing, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parsePathID(req.GetId(), msgListingNotFound)
	if err != nil {
		return nil, err
	}
	stage, err := stageParam(req.GetStage())
	if err != nil {
		return nil, err
	}
	l, err := i.Listings.Update(ctx, id, me, listings.UpdateParams{
		Title:        req.GetTitle(),
		Description:  req.GetDescription(),
		LookingFor:   req.GetLookingFor(),
		Tags:         req.GetTags(),
		Stage:        stage,
		Compensation: req.GetCompensation(),
	})
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return i.listingResponse(ctx, me, l)
}

// V1CloseListing closes a listing (author only, idempotent).
func (i *Implementation) V1CloseListing(ctx context.Context, req *hbcv1.V1CloseListingRequest) (*hbcv1.Listing, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parsePathID(req.GetId(), msgListingNotFound)
	if err != nil {
		return nil, err
	}
	l, err := i.Listings.Close(ctx, id, me)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return i.listingResponse(ctx, me, l)
}

// V1ListMyListings returns listings of the current user with application stats.
func (i *Implementation) V1ListMyListings(ctx context.Context, req *hbcv1.V1ListMyListingsRequest) (*hbcv1.V1ListMyListingsResponse, error) {
	me, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	st, err := listingStatusFilter(req.GetStatus())
	if err != nil {
		return nil, err
	}
	ls, err := i.Listings.ListByAuthor(ctx, me, st)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	items, err := i.composeListings(ctx, me, ls)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return &hbcv1.V1ListMyListingsResponse{Items: items}, nil
}

func (i *Implementation) listingResponse(ctx context.Context, me uuid.UUID, l *listings.Listing) (*hbcv1.Listing, error) {
	out, err := i.composeListing(ctx, me, l)
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return out, nil
}
