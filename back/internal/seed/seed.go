// Package seed loads the demo data of SPEC §6. It goes through the context
// services (not raw SQL), so every business rule applies: the closed listing gets
// its accepted application before Close, withdrawn is Apply + Withdraw, decisions
// are made by the listing author.
package seed

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/users"
)

// lockKey serializes seeding between instances starting at the same time.
const lockKey int64 = 0x6862635f73656564 // "hbc_seed"

// Deps are the context services the seed goes through.
type Deps struct {
	Users        users.Service
	Listings     listings.Service
	Applications applications.Service
}

// Run seeds the demo data when users.users is empty and skips otherwise. It holds
// a Postgres advisory lock for the duration, so concurrent starts seed once.
// It reports whether the data was loaded.
func Run(ctx context.Context, pool *pgxpool.Pool, log *zap.Logger, d Deps) (bool, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return false, fmt.Errorf("seed: acquire: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return false, fmt.Errorf("seed: lock: %w", err)
	}
	defer func() {
		// The connection goes back to the pool; release the lock even if ctx is done.
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", lockKey)
	}()

	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users.users)").Scan(&exists); err != nil {
		return false, fmt.Errorf("seed: check users: %w", err)
	}
	if exists {
		log.Info("demo data skipped: users table is not empty")
		return false, nil
	}

	if err := Load(ctx, d); err != nil {
		return false, err
	}
	log.Info("demo data loaded",
		zap.Int("users", len(demoUsers)),
		zap.Int("listings", len(demoListings)),
		zap.Int("applications", len(demoApplications)),
	)
	return true, nil
}

// Load creates the demo data unconditionally (Run checks for an empty database).
func Load(ctx context.Context, d Deps) error {
	userIDs := make(map[string]uuid.UUID, len(demoUsers))
	for _, u := range demoUsers {
		created, err := d.Users.Register(ctx, users.RegisterParams{
			Username: u.Username, Password: Password, DisplayName: u.DisplayName,
		})
		if err != nil {
			return fmt.Errorf("seed: register %s: %w", u.Username, err)
		}
		if _, err := d.Users.UpdateProfile(ctx, created.ID, users.UpdateProfileParams{
			DisplayName: u.DisplayName,
			Headline:    u.Headline,
			About:       u.About,
			Skills:      u.Skills,
			Telegram:    u.Telegram,
		}); err != nil {
			return fmt.Errorf("seed: profile %s: %w", u.Username, err)
		}
		userIDs[u.Username] = created.ID
	}

	listingIDs := make([]uuid.UUID, len(demoListings))
	for i, l := range demoListings {
		created, err := d.Listings.Create(ctx, listings.CreateParams{
			AuthorID:     userIDs[l.Author],
			Kind:         l.Kind,
			Title:        l.Title,
			Description:  l.Description,
			LookingFor:   l.LookingFor,
			Tags:         l.Tags,
			Stage:        l.Stage,
			Compensation: l.Compensation,
		})
		if err != nil {
			return fmt.Errorf("seed: listing %q: %w", l.Title, err)
		}
		listingIDs[i] = created.ID
	}

	for _, a := range demoApplications {
		l := demoListings[a.Listing]
		listingID, authorID, applicantID := listingIDs[a.Listing], userIDs[l.Author], userIDs[a.Applicant]
		app, err := d.Applications.Apply(ctx, listingID, applicantID, a.Message)
		if err != nil {
			return fmt.Errorf("seed: %s applies to %q: %w", a.Applicant, l.Title, err)
		}
		switch a.Outcome {
		case outcomeAccepted:
			_, err = d.Applications.Accept(ctx, app.ID, authorID)
		case outcomeDeclined:
			_, err = d.Applications.Decline(ctx, app.ID, authorID)
		case outcomeWithdrawn:
			_, err = d.Applications.Withdraw(ctx, app.ID, applicantID)
		}
		if err != nil {
			return fmt.Errorf("seed: %s on %s's application to %q: %w", a.Outcome, a.Applicant, l.Title, err)
		}
	}

	// Closed listings are closed last: they must accept their applications first.
	for i, l := range demoListings {
		if !l.Closed {
			continue
		}
		if _, err := d.Listings.Close(ctx, listingIDs[i], userIDs[l.Author]); err != nil {
			return fmt.Errorf("seed: close %q: %w", l.Title, err)
		}
	}
	return nil
}
