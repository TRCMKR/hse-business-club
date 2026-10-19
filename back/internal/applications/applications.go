// Package applications is the applications bounded context: responses to listings.
//
// This file is the port from ARCHITECTURE §2: applications/service implements
// Service, the BFF consumes it; ListingReader is implemented in internal/adapters.
// Signatures change only together with ARCHITECTURE.md.
package applications

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Status is the application lifecycle state.
type Status string // "pending" | "accepted" | "declined" | "withdrawn"

// Application statuses.
const (
	StatusPending   Status = "pending"
	StatusAccepted  Status = "accepted"
	StatusDeclined  Status = "declined"
	StatusWithdrawn Status = "withdrawn"
)

// Application is a response of a member to a listing.
type Application struct {
	ID              uuid.UUID
	ListingID       uuid.UUID
	ListingAuthorID uuid.UUID // денормализовано при создании
	ApplicantID     uuid.UUID
	Message         string
	Status          Status
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DecidedAt       *time.Time
}

// Stats are application counters of one listing.
type Stats struct{ Pending, Accepted, Declined, Total int }

// ListingInfo — что контексту нужно знать о публикации. Реализуется в internal/adapters поверх listings.Service.
type ListingInfo struct {
	ID       uuid.UUID
	AuthorID uuid.UUID
	IsOpen   bool
}

// ListingReader reads listings for the applications context.
type ListingReader interface {
	GetListing(ctx context.Context, id uuid.UUID) (ListingInfo, error) // KindNotFound
}

// Service is the applications context API. Errors are *apperr.Error.
type Service interface {
	// KindNotFound (публикации), KindPrecondition «Нельзя откликнуться на свою публикацию» / «Публикация закрыта»,
	// KindConflict «Вы уже откликались на эту публикацию», KindValidation.
	Apply(ctx context.Context, listingID, applicantID uuid.UUID, message string) (*Application, error)
	// Виден автору публикации и откликнувшемуся, остальным KindNotFound.
	Get(ctx context.Context, id, actorID uuid.UUID) (*Application, error)
	// actor = автор публикации, иначе KindNotFound. pending→accepted; accepted→accepted = no-op;
	// из declined/withdrawn — KindPrecondition «Решение по отклику уже принято».
	Accept(ctx context.Context, id, actorID uuid.UUID) (*Application, error)
	Decline(ctx context.Context, id, actorID uuid.UUID) (*Application, error)
	// actor = откликнувшийся, иначе KindNotFound. pending→withdrawn; withdrawn = no-op; иначе KindPrecondition.
	Withdraw(ctx context.Context, id, actorID uuid.UUID) (*Application, error)
	// actor должен быть автором публикации, иначе KindForbidden. status "" = все. Сортировка: pending первыми, затем created_at DESC.
	ListByListing(ctx context.Context, listingID, actorID uuid.UUID, status Status) ([]*Application, error)
	// Отправленные. created_at DESC.
	ListByApplicant(ctx context.Context, applicantID uuid.UUID, status Status) ([]*Application, error)
	// Полученные: все отклики на публикации автора. pending первыми, затем created_at DESC.
	ListReceived(ctx context.Context, authorID uuid.UUID, status Status) ([]*Application, error)
	// Не отозванный отклик applicant'а на каждую из публикаций (для ленты/деталей). Нет — нет ключа.
	GetMineForListings(ctx context.Context, applicantID uuid.UUID, listingIDs []uuid.UUID) (map[uuid.UUID]*Application, error)
	// Статистика по публикациям (для автора). Нет откликов — нулевая Stats присутствует.
	StatsForListings(ctx context.Context, listingIDs []uuid.UUID) (map[uuid.UUID]Stats, error)
}
