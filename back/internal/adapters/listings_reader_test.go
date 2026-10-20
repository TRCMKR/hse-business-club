package adapters_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/trcmkr/hse-business-club/back/internal/adapters"
	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/listings/mocks"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

func TestListingReader(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status listings.Status
		open   bool
	}{
		{"active", listings.StatusActive, true},
		{"closed", listings.StatusClosed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := mocks.NewMockService(t)
			l := &listings.Listing{ID: uuid.New(), AuthorID: uuid.New(), Status: tc.status}
			svc.EXPECT().Get(mock.Anything, l.ID).Return(l, nil).Once()

			info, err := adapters.NewListingReader(svc).GetListing(context.Background(), l.ID)
			require.NoError(t, err)
			assert.Equal(t, applications.ListingInfo{ID: l.ID, AuthorID: l.AuthorID, IsOpen: tc.open}, info)
		})
	}

	t.Run("not found passes through", func(t *testing.T) {
		svc := mocks.NewMockService(t)
		id := uuid.New()
		notFound := apperr.New(apperr.KindNotFound, "Публикация не найдена")
		svc.EXPECT().Get(mock.Anything, id).Return(nil, notFound).Once()

		info, err := adapters.NewListingReader(svc).GetListing(context.Background(), id)
		require.ErrorIs(t, err, notFound)
		assert.Equal(t, applications.ListingInfo{}, info)
	})
}
