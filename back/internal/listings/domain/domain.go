// Package domain holds the listings invariants: input validation and normalization
// (SPEC §2.4), feed filter defaults and lifecycle transition rules (SPEC §2.2–2.3).
// It is pure Go: no I/O, no pgx.
package domain

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

// Field length limits and feed paging (SPEC §2.4).
const (
	TitleMin           = 3
	TitleMax           = 120
	DescriptionMin     = 10
	DescriptionMax     = 5000
	LookingForMin      = 1
	LookingForMax      = 1000
	TagsMax            = 10
	TagMax             = 40
	CompensationMax    = 120
	QueryMax           = 100
	DefaultPage        = 1
	DefaultLimit       = 20
	MaxLimit           = 50
	fieldTitle         = "title"
	fieldDescription   = "description"
	fieldLookingFor    = "looking_for"
	fieldTags          = "tags"
	fieldStage         = "stage"
	fieldCompensation  = "compensation"
	fieldKind          = "kind"
	fieldQuery         = "query"
	fieldPage          = "page"
	fieldLimit         = "limit"
	fieldStatus        = "status"
	msgNotFound        = "Публикация не найдена"
	msgForbidden       = "Недостаточно прав"
	msgClosed          = "Публикация закрыта"
	msgTitle           = "Название: от 3 до 120 символов"
	msgDescription     = "Описание: от 10 до 5000 символов"
	msgLookingFor      = "Кого ищете: от 1 до 1000 символов"
	msgTagsCount       = "Теги: не больше 10"
	msgTagLen          = "Теги: каждый не длиннее 40 символов"
	msgTagsDup         = "Теги: без повторов"
	msgStageRequired   = "Стадия: укажите стадию"
	msgStageInvalid    = "Стадия: недопустимое значение"
	msgStageStartup    = "Стадия: только для стартапа"
	msgCompensationLen = "Условия: не больше 120 символов"
	msgCompensationVac = "Условия: только для вакансии"
	msgKind            = "Вид публикации: выберите стартап или вакансию"
	msgQuery           = "Поиск: не больше 100 символов"
	msgPage            = "Страница: не меньше 1"
	msgLimit           = "Размер страницы: от 1 до 50"
	msgStatus          = "Статус: недопустимое значение"
)

// ErrNotFound returns the KindNotFound error for a missing listing.
func ErrNotFound() *apperr.Error { return apperr.New(apperr.KindNotFound, msgNotFound) }

// ErrForbidden returns the KindForbidden error for a non-author actor.
func ErrForbidden() *apperr.Error { return apperr.New(apperr.KindForbidden, msgForbidden) }

// ErrClosed returns the KindPrecondition error for an edit of a closed listing.
func ErrClosed() *apperr.Error { return apperr.New(apperr.KindPrecondition, msgClosed) }

// fields are the editable listing fields shared by create and update.
type fields struct {
	title, description, lookingFor string
	tags                           []string
	stage                          listings.Stage
	compensation                   string
}

// NormalizeCreate validates p and returns it with trimmed strings and normalized tags.
func NormalizeCreate(p listings.CreateParams) (listings.CreateParams, error) {
	if !ValidKind(p.Kind) {
		return p, apperr.Validation(fieldKind, msgKind)
	}
	f, err := normalize(p.Kind, fields{p.Title, p.Description, p.LookingFor, p.Tags, p.Stage, p.Compensation})
	if err != nil {
		return p, err
	}
	p.Title, p.Description, p.LookingFor = f.title, f.description, f.lookingFor
	p.Tags, p.Stage, p.Compensation = f.tags, f.stage, f.compensation
	return p, nil
}

// NormalizeUpdate validates p against the (immutable) listing kind and returns it normalized.
func NormalizeUpdate(kind listings.Kind, p listings.UpdateParams) (listings.UpdateParams, error) {
	if !ValidKind(kind) {
		return p, apperr.Validation(fieldKind, msgKind)
	}
	f, err := normalize(kind, fields{p.Title, p.Description, p.LookingFor, p.Tags, p.Stage, p.Compensation})
	if err != nil {
		return p, err
	}
	p.Title, p.Description, p.LookingFor = f.title, f.description, f.lookingFor
	p.Tags, p.Stage, p.Compensation = f.tags, f.stage, f.compensation
	return p, nil
}

func normalize(kind listings.Kind, f fields) (fields, error) {
	f.title = strings.TrimSpace(f.title)
	if !runeLenIn(f.title, TitleMin, TitleMax) {
		return f, apperr.Validation(fieldTitle, msgTitle)
	}
	f.description = strings.TrimSpace(f.description)
	if !runeLenIn(f.description, DescriptionMin, DescriptionMax) {
		return f, apperr.Validation(fieldDescription, msgDescription)
	}
	f.lookingFor = strings.TrimSpace(f.lookingFor)
	if !runeLenIn(f.lookingFor, LookingForMin, LookingForMax) {
		return f, apperr.Validation(fieldLookingFor, msgLookingFor)
	}
	tags, err := NormalizeTags(f.tags)
	if err != nil {
		return f, err
	}
	f.tags = tags

	switch kind {
	case listings.KindStartup:
		if f.stage == "" {
			return f, apperr.Validation(fieldStage, msgStageRequired)
		}
		if !ValidStage(f.stage) {
			return f, apperr.Validation(fieldStage, msgStageInvalid)
		}
	default: // vacancy
		if f.stage != "" {
			return f, apperr.Validation(fieldStage, msgStageStartup)
		}
	}

	f.compensation = strings.TrimSpace(f.compensation)
	if kind == listings.KindStartup && f.compensation != "" {
		return f, apperr.Validation(fieldCompensation, msgCompensationVac)
	}
	if utf8.RuneCountInString(f.compensation) > CompensationMax {
		return f, apperr.Validation(fieldCompensation, msgCompensationLen)
	}
	return f, nil
}

// NormalizeTags trims tags, drops empty ones and checks count, length and
// case-insensitive uniqueness. The result is never nil; order is preserved.
func NormalizeTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if utf8.RuneCountInString(t) > TagMax {
			return nil, apperr.Validation(fieldTags, msgTagLen)
		}
		key := strings.ToLower(t)
		if _, dup := seen[key]; dup {
			return nil, apperr.Validation(fieldTags, msgTagsDup)
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	if len(out) > TagsMax {
		return nil, apperr.Validation(fieldTags, msgTagsCount)
	}
	return out, nil
}

// NormalizeFilter applies feed defaults (page 1, limit 20) to zero values and validates
// kind, page, limit and query. The query is trimmed.
func NormalizeFilter(f listings.ListFilter) (listings.ListFilter, error) {
	if f.Kind != "" && !ValidKind(f.Kind) {
		return f, apperr.Validation(fieldKind, msgKind)
	}
	f.Query = strings.TrimSpace(f.Query)
	if utf8.RuneCountInString(f.Query) > QueryMax {
		return f, apperr.Validation(fieldQuery, msgQuery)
	}
	if f.Page == 0 {
		f.Page = DefaultPage
	}
	if f.Page < 1 {
		return f, apperr.Validation(fieldPage, msgPage)
	}
	if f.Limit == 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit < 1 || f.Limit > MaxLimit {
		return f, apperr.Validation(fieldLimit, msgLimit)
	}
	return f, nil
}

// Offset returns the row offset of a normalized filter's page.
func Offset(f listings.ListFilter) int {
	return (f.Page - 1) * f.Limit
}

// ValidateStatusFilter accepts "" (all), active and closed.
func ValidateStatusFilter(s listings.Status) error {
	switch s {
	case "", listings.StatusActive, listings.StatusClosed:
		return nil
	default:
		return apperr.Validation(fieldStatus, msgStatus)
	}
}

// ValidKind reports whether k is startup or vacancy.
func ValidKind(k listings.Kind) bool {
	return k == listings.KindStartup || k == listings.KindVacancy
}

// ValidStage reports whether s is one of the startup stages (not "").
func ValidStage(s listings.Stage) bool {
	switch s {
	case listings.StageIdea, listings.StagePrototype, listings.StageMVP, listings.StageLaunched:
		return true
	default:
		return false
	}
}

// CheckAuthor returns KindForbidden unless actorID is the listing author.
func CheckAuthor(l *listings.Listing, actorID uuid.UUID) error {
	if l.AuthorID != actorID {
		return ErrForbidden()
	}
	return nil
}

// CanEdit returns KindPrecondition «Публикация закрыта» unless the listing is active.
func CanEdit(l *listings.Listing) error {
	if l.Status != listings.StatusActive {
		return ErrClosed()
	}
	return nil
}

// NeedsClose reports whether Close has to change the listing; closing a closed
// listing is a no-op that returns the current state (idempotency, SPEC §2.3).
func NeedsClose(l *listings.Listing) bool {
	return l.Status == listings.StatusActive
}

func runeLenIn(s string, minLen, maxLen int) bool {
	n := utf8.RuneCountInString(s)
	return n >= minLen && n <= maxLen
}

// ErrNotActive is returned by the repository when a conditional update (edit, close)
// matched no active listing: the listing is missing or already closed. The service
// re-reads the listing to tell the cases apart. It never leaves the context.
var ErrNotActive = errors.New("listings: listing is missing or not active")
