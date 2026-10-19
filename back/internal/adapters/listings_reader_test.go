package adapters_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/adapters"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

// stubListings implements only Get; other methods panic through the nil embedded interface.
type stubListings struct {
	listings.Service
	items map[uuid.UUID]*listings.Listing
}

func (s stubListings) Get(_ context.Context, id uuid.UUID) (*listings.Listing, error) {
	if l, ok := s.items[id]; ok {
		return l, nil
	}
	return nil, apperr.New(apperr.KindNotFound, "Публикация не найдена")
}

func TestListingReader(t *testing.T) {
	active := &listings.Listing{ID: uuid.New(), AuthorID: uuid.New(), Status: listings.StatusActive}
	closed := &listings.Listing{ID: uuid.New(), AuthorID: uuid.New(), Status: listings.StatusClosed}
	r := adapters.NewListingReader(stubListings{items: map[uuid.UUID]*listings.Listing{
		active.ID: active, closed.ID: closed,
	}})

	for _, tc := range []struct {
		name string
		l    *listings.Listing
		open bool
	}{
		{"active", active, true},
		{"closed", closed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info, err := r.GetListing(context.Background(), tc.l.ID)
			if err != nil {
				t.Fatalf("GetListing: %v", err)
			}
			if info.ID != tc.l.ID || info.AuthorID != tc.l.AuthorID || info.IsOpen != tc.open {
				t.Fatalf("got %+v, want id %s author %s open %v", info, tc.l.ID, tc.l.AuthorID, tc.open)
			}
		})
	}

	t.Run("not found passes through", func(t *testing.T) {
		_, err := r.GetListing(context.Background(), uuid.New())
		if apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("got %v, want KindNotFound", err)
		}
	})
}
