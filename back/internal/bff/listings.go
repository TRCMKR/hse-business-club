package bff

import (
	"context"

	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// V1ListListings is not implemented yet (B4).
func (i *Implementation) V1ListListings(_ context.Context, _ *hbcv1.V1ListListingsRequest) (*hbcv1.V1ListListingsResponse, error) {
	return nil, errNotImplemented
}

// V1CreateListing is not implemented yet (B4).
func (i *Implementation) V1CreateListing(_ context.Context, _ *hbcv1.V1CreateListingRequest) (*hbcv1.Listing, error) {
	return nil, errNotImplemented
}

// V1GetListing is not implemented yet (B4).
func (i *Implementation) V1GetListing(_ context.Context, _ *hbcv1.V1GetListingRequest) (*hbcv1.Listing, error) {
	return nil, errNotImplemented
}

// V1UpdateListing is not implemented yet (B4).
func (i *Implementation) V1UpdateListing(_ context.Context, _ *hbcv1.V1UpdateListingRequest) (*hbcv1.Listing, error) {
	return nil, errNotImplemented
}

// V1CloseListing is not implemented yet (B4).
func (i *Implementation) V1CloseListing(_ context.Context, _ *hbcv1.V1CloseListingRequest) (*hbcv1.Listing, error) {
	return nil, errNotImplemented
}

// V1ListMyListings is not implemented yet (B4).
func (i *Implementation) V1ListMyListings(_ context.Context, _ *hbcv1.V1ListMyListingsRequest) (*hbcv1.V1ListMyListingsResponse, error) {
	return nil, errNotImplemented
}
