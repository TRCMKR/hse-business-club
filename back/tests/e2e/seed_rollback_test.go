//go:build e2e

package e2e

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/trcmkr/hse-business-club/back/internal/adapters"
	appspg "github.com/trcmkr/hse-business-club/back/internal/applications/postgres"
	appssvc "github.com/trcmkr/hse-business-club/back/internal/applications/service"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	listingspg "github.com/trcmkr/hse-business-club/back/internal/listings/postgres"
	listingssvc "github.com/trcmkr/hse-business-club/back/internal/listings/service"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob/memory"
	"github.com/trcmkr/hse-business-club/back/internal/seed"
	userspg "github.com/trcmkr/hse-business-club/back/internal/users/postgres"
	userssvc "github.com/trcmkr/hse-business-club/back/internal/users/service"
)

// failingClose makes the last step of the seed (closing listing №6) fail.
type failingClose struct{ listings.Service }

var errClose = errors.New("close failed")

func (failingClose) Close(context.Context, uuid.UUID, uuid.UUID) (*listings.Listing, error) {
	return nil, errClose
}

// A seed that fails in the middle deletes what it has created, so the next start
// seeds again instead of skipping with partial data.
func TestSeedRollsBackOnError(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	usersSvc := userssvc.New(userspg.New(pool), userspg.NewRefresh(pool), userssvc.Bcrypt, memory.New())
	listingsSvc := listingssvc.New(listingspg.New(pool))
	appsSvc := appssvc.New(appspg.New(pool), adapters.NewListingReader(listingsSvc))

	loaded, err := seed.Run(ctx, pool, zap.NewNop(), seed.Deps{
		Users: usersSvc, Listings: failingClose{listingsSvc}, Applications: appsSvc,
	})
	if loaded || !errors.Is(err, errClose) {
		t.Fatalf("Run = %v, %v; want false, errClose", loaded, err)
	}
	for _, table := range strings.Split(strings.TrimPrefix(truncateSQL, "TRUNCATE "), ", ") {
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("%s has %d rows after the rollback, want 0", table, n)
		}
	}

	// The next start loads the full data set.
	loaded, err = seed.Run(ctx, pool, zap.NewNop(), seed.Deps{
		Users: usersSvc, Listings: listingsSvc, Applications: appsSvc,
	})
	if !loaded || err != nil {
		t.Fatalf("second Run = %v, %v; want true, nil", loaded, err)
	}
}
