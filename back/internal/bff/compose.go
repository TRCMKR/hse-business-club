package bff

import (
	"context"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// Response composition across contexts. Every helper makes a fixed number of batch
// port calls regardless of the number of items: no calls in loops.

// idSet collects unique ids preserving the first-seen order.
type idSet struct {
	seen map[uuid.UUID]struct{}
	ids  []uuid.UUID
}

func (s *idSet) add(id uuid.UUID) {
	if s.seen == nil {
		s.seen = make(map[uuid.UUID]struct{})
	}
	if _, ok := s.seen[id]; ok {
		return
	}
	s.seen[id] = struct{}{}
	s.ids = append(s.ids, id)
}

// usersProto loads users in one batch and maps them to proto.
func (i *Implementation) usersProto(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*hbcv1.User, error) {
	out := make(map[uuid.UUID]*hbcv1.User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	found, err := i.Users.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	for id, u := range found {
		out[id] = userProto(u)
	}
	return out, nil
}

// composeListings builds proto listings for the caller me:
//   - Users.GetMany for authors (and me, the applicant of my_application);
//   - Applications.GetMineForListings for listings of other authors;
//   - Applications.StatsForListings for my own listings (stats is null for others).
//
// At most three port calls for any number of listings.
func (i *Implementation) composeListings(ctx context.Context, me uuid.UUID, ls []*listings.Listing) ([]*hbcv1.Listing, error) {
	out := make([]*hbcv1.Listing, 0, len(ls))
	if len(ls) == 0 {
		return out, nil
	}

	var userIDs, foreignIDs, myIDs idSet
	for _, l := range ls {
		userIDs.add(l.AuthorID)
		if l.AuthorID == me {
			myIDs.add(l.ID)
		} else {
			foreignIDs.add(l.ID)
		}
	}
	if len(foreignIDs.ids) > 0 {
		userIDs.add(me)
	}

	people, err := i.usersProto(ctx, userIDs.ids)
	if err != nil {
		return nil, err
	}
	mine := map[uuid.UUID]*applications.Application{}
	if len(foreignIDs.ids) > 0 {
		if mine, err = i.Applications.GetMineForListings(ctx, me, foreignIDs.ids); err != nil {
			return nil, err
		}
	}
	stats := map[uuid.UUID]applications.Stats{}
	if len(myIDs.ids) > 0 {
		if stats, err = i.Applications.StatsForListings(ctx, myIDs.ids); err != nil {
			return nil, err
		}
	}

	for _, l := range ls {
		author := people[l.AuthorID]
		var (
			myApp *hbcv1.Application
			st    *hbcv1.ListingStats
		)
		if l.AuthorID == me {
			st = statsProto(stats[l.ID]) // absent key = zero stats
		} else if a, ok := mine[l.ID]; ok {
			myApp = applicationProto(a, listingSummaryProto(l, author), people[me])
		}
		out = append(out, listingProto(l, me, author, myApp, st))
	}
	return out, nil
}

// composeListing is composeListings for a single listing.
func (i *Implementation) composeListing(ctx context.Context, me uuid.UUID, l *listings.Listing) (*hbcv1.Listing, error) {
	out, err := i.composeListings(ctx, me, []*listings.Listing{l})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

// composeApplications builds proto applications with listing summaries and users:
// Listings.GetMany for listings, then Users.GetMany for applicants and listing
// authors — two port calls for any number of applications. A listing or user that
// is missing in the batch result (should not happen) leaves the field null.
func (i *Implementation) composeApplications(ctx context.Context, apps []*applications.Application) ([]*hbcv1.Application, error) {
	out := make([]*hbcv1.Application, 0, len(apps))
	if len(apps) == 0 {
		return out, nil
	}

	var listingIDs, userIDs idSet
	for _, a := range apps {
		listingIDs.add(a.ListingID)
		userIDs.add(a.ApplicantID)
	}
	ls, err := i.Listings.GetMany(ctx, listingIDs.ids)
	if err != nil {
		return nil, err
	}
	for _, l := range ls {
		userIDs.add(l.AuthorID)
	}
	people, err := i.usersProto(ctx, userIDs.ids)
	if err != nil {
		return nil, err
	}

	for _, a := range apps {
		var summary *hbcv1.ListingSummary
		if l, ok := ls[a.ListingID]; ok {
			summary = listingSummaryProto(l, people[l.AuthorID])
		}
		out = append(out, applicationProto(a, summary, people[a.ApplicantID]))
	}
	return out, nil
}

// composeApplication is composeApplications for a single application.
func (i *Implementation) composeApplication(ctx context.Context, a *applications.Application) (*hbcv1.Application, error) {
	out, err := i.composeApplications(ctx, []*applications.Application{a})
	if err != nil {
		return nil, err
	}
	return out[0], nil
}
