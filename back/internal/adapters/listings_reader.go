// Package adapters glues bounded contexts together: each adapter implements a port
// of one context on top of the service of another (ARCHITECTURE §1). When a context
// moves into its own process, the in-process adapter is replaced by a gRPC client.
package adapters

import (
	"context"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
)

// ListingReader implements applications.ListingReader over listings.Service.
type ListingReader struct {
	listings listings.Service
}

var _ applications.ListingReader = (*ListingReader)(nil)

// NewListingReader creates the adapter.
func NewListingReader(s listings.Service) *ListingReader {
	return &ListingReader{listings: s}
}

// GetListing returns what the applications context needs to know about a listing.
// Errors of listings.Service (KindNotFound «Публикация не найдена») pass through as is.
func (r *ListingReader) GetListing(ctx context.Context, id uuid.UUID) (applications.ListingInfo, error) {
	l, err := r.listings.Get(ctx, id)
	if err != nil {
		return applications.ListingInfo{}, err
	}
	return applications.ListingInfo{
		ID:       l.ID,
		AuthorID: l.AuthorID,
		IsOpen:   l.Status == listings.StatusActive,
	}, nil
}
