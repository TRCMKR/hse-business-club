// Package service implements applications.Service: the rules of SPEC §2.2–2.3 on top of
// the repository and the applications.ListingReader port.
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/applications/domain"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

// Repository is the storage the service needs; postgres.Repo implements it.
type Repository interface {
	// Create inserts a pending application; KindConflict for a second live one.
	Create(ctx context.Context, a *applications.Application) (*applications.Application, error)
	// GetByID — KindNotFound if there is none.
	GetByID(ctx context.Context, id uuid.UUID) (*applications.Application, error)
	// Update locks the row, calls fn and applies a non-nil change in one transaction;
	// nil change returns the current state without writing. KindNotFound if there is none.
	Update(ctx context.Context, id uuid.UUID, fn func(cur *applications.Application) (*domain.StatusChange, error)) (*applications.Application, error)
	ListByListing(ctx context.Context, listingID uuid.UUID, status applications.Status) ([]*applications.Application, error)
	ListByApplicant(ctx context.Context, applicantID uuid.UUID, status applications.Status) ([]*applications.Application, error)
	ListByListingAuthor(ctx context.Context, authorID uuid.UUID, status applications.Status) ([]*applications.Application, error)
	GetMineForListings(ctx context.Context, applicantID uuid.UUID, listingIDs []uuid.UUID) (map[uuid.UUID]*applications.Application, error)
	StatsForListings(ctx context.Context, listingIDs []uuid.UUID) (map[uuid.UUID]applications.Stats, error)
}

// Service implements applications.Service.
type Service struct {
	repo     Repository
	listings applications.ListingReader
	now      func() time.Time
}

var _ applications.Service = (*Service)(nil)

// Option configures Service.
type Option func(*Service)

// WithClock sets the clock used for decided_at (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New creates the service.
func New(repo Repository, listings applications.ListingReader, opts ...Option) *Service {
	s := &Service{repo: repo, listings: listings, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Apply creates a pending application of applicantID to listingID.
func (s *Service) Apply(ctx context.Context, listingID, applicantID uuid.UUID, message string) (*applications.Application, error) {
	msg, err := domain.NormalizeMessage(message)
	if err != nil {
		return nil, err
	}
	info, err := s.listings.GetListing(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if info.AuthorID == applicantID {
		return nil, apperr.New(apperr.KindPrecondition, domain.MsgOwnListing)
	}
	if !info.IsOpen {
		return nil, apperr.New(apperr.KindPrecondition, domain.MsgListingClosed)
	}
	// listing_author_id is copied here (ARCHITECTURE §1, rule 3): the author never changes.
	return s.repo.Create(ctx, &applications.Application{
		ListingID:       info.ID,
		ListingAuthorID: info.AuthorID,
		ApplicantID:     applicantID,
		Message:         msg,
	})
}

// Get returns the application to the listing author or the applicant; others get KindNotFound.
func (s *Service) Get(ctx context.Context, id, actorID uuid.UUID) (*applications.Application, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !isParty(a, actorID) {
		return nil, notFound()
	}
	return a, nil
}

// Accept moves a pending application to accepted (listing author only).
func (s *Service) Accept(ctx context.Context, id, actorID uuid.UUID) (*applications.Application, error) {
	return s.transition(ctx, id, actorID, domain.ActionAccept)
}

// Decline moves a pending application to declined (listing author only).
func (s *Service) Decline(ctx context.Context, id, actorID uuid.UUID) (*applications.Application, error) {
	return s.transition(ctx, id, actorID, domain.ActionDecline)
}

// Withdraw moves a pending application to withdrawn (applicant only).
func (s *Service) Withdraw(ctx context.Context, id, actorID uuid.UUID) (*applications.Application, error) {
	return s.transition(ctx, id, actorID, domain.ActionWithdraw)
}

// transition runs action under the row lock. A stranger (neither the listing author nor
// the applicant) gets KindNotFound, the other party gets KindForbidden (SPEC §2.3).
func (s *Service) transition(ctx context.Context, id, actorID uuid.UUID, action domain.Action) (*applications.Application, error) {
	return s.repo.Update(ctx, id, func(cur *applications.Application) (*domain.StatusChange, error) {
		if !isParty(cur, actorID) {
			return nil, notFound()
		}
		allowed := cur.ApplicantID
		if action.IsDecision() {
			allowed = cur.ListingAuthorID
		}
		if actorID != allowed {
			return nil, forbidden()
		}
		to, noop, err := domain.Transition(cur.Status, action)
		if err != nil || noop {
			return nil, err
		}
		change := &domain.StatusChange{Status: to}
		if action.IsDecision() {
			at := s.now().UTC().Truncate(time.Microsecond)
			change.DecidedAt = &at
		}
		return change, nil
	})
}

// ListByListing returns applications to a listing; only its author may see them.
func (s *Service) ListByListing(ctx context.Context, listingID, actorID uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	if err := domain.ValidateStatusFilter(status); err != nil {
		return nil, err
	}
	// Checked through the port, not through listing_author_id of the rows:
	// with zero applications there would be nothing to check.
	info, err := s.listings.GetListing(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if info.AuthorID != actorID {
		return nil, forbidden()
	}
	return s.repo.ListByListing(ctx, listingID, status)
}

// ListByApplicant returns applications sent by applicantID, newest first.
func (s *Service) ListByApplicant(ctx context.Context, applicantID uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	if err := domain.ValidateStatusFilter(status); err != nil {
		return nil, err
	}
	return s.repo.ListByApplicant(ctx, applicantID, status)
}

// ListReceived returns applications to all listings of authorID, pending first.
func (s *Service) ListReceived(ctx context.Context, authorID uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	if err := domain.ValidateStatusFilter(status); err != nil {
		return nil, err
	}
	return s.repo.ListByListingAuthor(ctx, authorID, status)
}

// GetMineForListings returns the live application of applicantID for each listing that has one.
func (s *Service) GetMineForListings(ctx context.Context, applicantID uuid.UUID, listingIDs []uuid.UUID) (map[uuid.UUID]*applications.Application, error) {
	return s.repo.GetMineForListings(ctx, applicantID, listingIDs)
}

// StatsForListings returns Stats for every requested listing (zero when there are no applications).
func (s *Service) StatsForListings(ctx context.Context, listingIDs []uuid.UUID) (map[uuid.UUID]applications.Stats, error) {
	return s.repo.StatsForListings(ctx, listingIDs)
}

func isParty(a *applications.Application, actorID uuid.UUID) bool {
	return actorID == a.ListingAuthorID || actorID == a.ApplicantID
}

func notFound() error  { return apperr.New(apperr.KindNotFound, domain.MsgNotFound) }
func forbidden() error { return apperr.New(apperr.KindForbidden, domain.MsgForbidden) }
