// Package listings is the listings bounded context: startups and vacancies.
//
// This file is the port from ARCHITECTURE §2: listings/service implements Service,
// the BFF consumes it. Signatures change only together with ARCHITECTURE.md.
package listings

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Kind is the listing type.
type Kind string // "startup" | "vacancy"

// Status is the listing lifecycle state.
type Status string // "active" | "closed"

// Stage is the startup stage.
type Stage string // "" | "idea" | "prototype" | "mvp" | "launched"

// Listing kinds, statuses and startup stages.
const (
	KindStartup    Kind   = "startup"
	KindVacancy    Kind   = "vacancy"
	StatusActive   Status = "active"
	StatusClosed   Status = "closed"
	StageIdea      Stage  = "idea"
	StagePrototype Stage  = "prototype"
	StageMVP       Stage  = "mvp"
	StageLaunched  Stage  = "launched"
)

// Listing is a startup or a vacancy.
type Listing struct {
	ID           uuid.UUID
	AuthorID     uuid.UUID
	Kind         Kind
	Title        string
	Description  string
	LookingFor   string
	Tags         []string
	Stage        Stage  // только для startup, иначе ""
	Compensation string // только для vacancy, иначе ""
	Status       Status
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ClosedAt     *time.Time
}

// CreateParams are the inputs of Service.Create.
type CreateParams struct {
	AuthorID                       uuid.UUID
	Kind                           Kind
	Title, Description, LookingFor string
	Tags                           []string
	Stage                          Stage
	Compensation                   string
}

// UpdateParams are the editable listing fields (full replacement; Kind is immutable).
type UpdateParams struct {
	Title, Description, LookingFor string
	Tags                           []string
	Stage                          Stage
	Compensation                   string
}

// ListFilter filters the feed of active listings.
type ListFilter struct {
	Kind  Kind   // "" = все
	Query string // "" = без поиска
	Page  int    // с 1
	Limit int    // 1..50
}

// Page is one page of the feed.
type Page struct {
	Items              []*Listing
	Total, Page, Limit int
}

// Service is the listings context API. Errors are *apperr.Error.
type Service interface {
	Create(ctx context.Context, p CreateParams) (*Listing, error) // KindValidation
	Get(ctx context.Context, id uuid.UUID) (*Listing, error)      // KindNotFound; closed тоже возвращается
	GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*Listing, error)
	ListActive(ctx context.Context, f ListFilter) (*Page, error)                             // KindValidation для page/limit/query
	ListByAuthor(ctx context.Context, authorID uuid.UUID, status Status) ([]*Listing, error) // status "" = все
	Update(ctx context.Context, id, actorID uuid.UUID, p UpdateParams) (*Listing, error)     // KindNotFound, KindForbidden, KindPrecondition «Публикация закрыта»
	Close(ctx context.Context, id, actorID uuid.UUID) (*Listing, error)                      // идемпотентно; KindNotFound, KindForbidden
}
