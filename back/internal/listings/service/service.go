// Package service implements listings.Service on top of a Repository.
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/listings/domain"
)

// Repository is the storage the service needs; postgres.Repository implements it.
// GetByID returns domain.ErrNotFound() for a missing id; Update and Close return
// domain.ErrNotActive when no active listing matched. Inputs are already normalized.
type Repository interface {
	Create(ctx context.Context, p listings.CreateParams) (*listings.Listing, error)
	GetByID(ctx context.Context, id uuid.UUID) (*listings.Listing, error)
	GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*listings.Listing, error)
	ListActive(ctx context.Context, f listings.ListFilter) ([]*listings.Listing, int, error)
	ListByAuthor(ctx context.Context, authorID uuid.UUID, status listings.Status) ([]*listings.Listing, error)
	Update(ctx context.Context, id uuid.UUID, p listings.UpdateParams) (*listings.Listing, error)
	Close(ctx context.Context, id uuid.UUID) (*listings.Listing, error)
}

// Service implements listings.Service.
type Service struct {
	repo Repository
}

var _ listings.Service = (*Service)(nil)

// New creates the listings service.
func New(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates p and stores an active listing.
func (s *Service) Create(ctx context.Context, p listings.CreateParams) (*listings.Listing, error) {
	p, err := domain.NormalizeCreate(p)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, p)
}

// Get returns a listing in any status.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*listings.Listing, error) {
	return s.repo.GetByID(ctx, id)
}

// GetMany returns the found listings; missing ids are absent from the map.
func (s *Service) GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*listings.Listing, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]*listings.Listing{}, nil
	}
	return s.repo.GetMany(ctx, ids)
}

// ListActive returns one page of the feed; zero Page/Limit get the defaults 1/20.
func (s *Service) ListActive(ctx context.Context, f listings.ListFilter) (*listings.Page, error) {
	f, err := domain.NormalizeFilter(f)
	if err != nil {
		return nil, err
	}
	items, total, err := s.repo.ListActive(ctx, f)
	if err != nil {
		return nil, err
	}
	return &listings.Page{Items: items, Total: total, Page: f.Page, Limit: f.Limit}, nil
}

// ListByAuthor returns the author's listings, newest first; status "" = all.
func (s *Service) ListByAuthor(ctx context.Context, authorID uuid.UUID, status listings.Status) ([]*listings.Listing, error) {
	if err := domain.ValidateStatusFilter(status); err != nil {
		return nil, err
	}
	return s.repo.ListByAuthor(ctx, authorID, status)
}

// Update replaces the editable fields. Only the author may edit, only an active listing.
// Checks go in order: existence, authorship, status, then input validation.
func (s *Service) Update(ctx context.Context, id, actorID uuid.UUID, p listings.UpdateParams) (*listings.Listing, error) {
	l, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := domain.CheckAuthor(l, actorID); err != nil {
		return nil, err
	}
	if err := domain.CanEdit(l); err != nil {
		return nil, err
	}
	p, err = domain.NormalizeUpdate(l.Kind, p)
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.Update(ctx, id, p)
	if errors.Is(err, domain.ErrNotActive) {
		// Closed concurrently since the read above.
		if _, gerr := s.repo.GetByID(ctx, id); gerr != nil {
			return nil, gerr
		}
		return nil, domain.ErrClosed()
	}
	return updated, err
}

// Close closes the listing. Only the author may close; closing a closed listing
// returns its current state.
func (s *Service) Close(ctx context.Context, id, actorID uuid.UUID) (*listings.Listing, error) {
	l, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := domain.CheckAuthor(l, actorID); err != nil {
		return nil, err
	}
	if !domain.NeedsClose(l) {
		return l, nil
	}
	closed, err := s.repo.Close(ctx, id)
	if errors.Is(err, domain.ErrNotActive) {
		// Closed concurrently: still idempotent.
		return s.repo.GetByID(ctx, id)
	}
	return closed, err
}
