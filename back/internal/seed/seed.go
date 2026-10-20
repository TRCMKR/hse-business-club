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
//
// The services commit each call on their own, so a failure in the middle would
// leave partial data and the next start would skip the seed (users is no longer
// empty). Run therefore deletes everything it has created before returning the
// error: the next start seeds again from scratch.
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

	var c created
	if err := load(ctx, d, &c); err != nil {
		if rbErr := c.rollback(context.WithoutCancel(ctx), conn); rbErr != nil {
			log.Error("demo data rollback failed: run with an empty database (make reset)", zap.Error(rbErr))
		} else {
			log.Warn("demo data rolled back after an error", zap.Error(err))
		}
		return false, err
	}
	log.Info("demo data loaded",
		zap.Int("users", len(demoUsers)),
		zap.Int("listings", len(demoListings)),
		zap.Int("applications", len(demoApplications)),
	)
	return true, nil
}

// created collects the ids of the records made by load, for the rollback.
type created struct {
	users, listings []uuid.UUID
}

// rollback deletes the created records in one transaction. Applications of the
// seed are all on the seed listings, so they are found by listing_id.
func (c *created) rollback(ctx context.Context, conn *pgxpool.Conn) error {
	if len(c.users) == 0 && len(c.listings) == 0 {
		return nil
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("seed: rollback: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, q := range []struct {
		sql string
		ids []uuid.UUID
	}{
		{"DELETE FROM applications.applications WHERE listing_id = ANY($1)", c.listings},
		{"DELETE FROM listings.listings WHERE id = ANY($1)", c.listings},
		{"DELETE FROM users.users WHERE id = ANY($1)", c.users},
	} {
		if _, err := tx.Exec(ctx, q.sql, q.ids); err != nil {
			return fmt.Errorf("seed: rollback: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("seed: rollback: %w", err)
	}
	return nil
}

// Load creates the demo data unconditionally (Run checks for an empty database
// and rolls back a partial load).
func Load(ctx context.Context, d Deps) error {
	return load(ctx, d, &created{})
}

func load(ctx context.Context, d Deps, c *created) error {
	userIDs := make(map[string]uuid.UUID, len(demoUsers))
	for _, u := range demoUsers {
		created, err := d.Users.Register(ctx, users.RegisterParams{
			Username: u.Username, Password: Password, DisplayName: u.DisplayName,
		})
		if err != nil {
			return fmt.Errorf("seed: register %s: %w", u.Username, err)
		}
		c.users = append(c.users, created.ID)
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
		c.listings = append(c.listings, created.ID)
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
