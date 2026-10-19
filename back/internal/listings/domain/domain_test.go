package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/listings/domain"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

// Field names of validation errors.
const (
	fKind         = "kind"
	fTitle        = "title"
	fDescription  = "description"
	fLookingFor   = "looking_for"
	fTags         = "tags"
	fStage        = "stage"
	fCompensation = "compensation"
	fQuery        = "query"
	fPage         = "page"
	fLimit        = "limit"
)

// Messages repeated across tables.
const (
	msgKind  = "Вид публикации: выберите стартап или вакансию"
	msgTitle = "Название: от 3 до 120 символов"
)

func validStartup() listings.CreateParams {
	return listings.CreateParams{
		AuthorID:    uuid.New(),
		Kind:        listings.KindStartup,
		Title:       "Сервис аренды",
		Description: "Описание стартапа на десять+ символов",
		LookingFor:  "Бэкенд-разработчик",
		Tags:        []string{"Go", "Postgres"},
		Stage:       listings.StageMVP,
	}
}

func validVacancy() listings.CreateParams {
	p := validStartup()
	p.Kind = listings.KindVacancy
	p.Stage = ""
	p.Compensation = "Доля 5%"
	return p
}

// wantValidation checks that err is a KindValidation error on field with msg.
func wantValidation(t *testing.T, err error, field, msg string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *apperr.Error", err)
	}
	if ae.Kind != apperr.KindValidation || ae.Field != field || ae.Msg != msg {
		t.Fatalf("err = {%v %q %q}, want {validation %q %q}", ae.Kind, ae.Field, ae.Msg, field, msg)
	}
}

func TestNormalizeCreate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(p *listings.CreateParams)
		field  string // "" = valid
		msg    string
	}{
		{"valid startup", func(*listings.CreateParams) {}, "", ""},
		{"valid vacancy", func(p *listings.CreateParams) { *p = validVacancy() }, "", ""},
		{"vacancy without compensation", func(p *listings.CreateParams) { *p = validVacancy(); p.Compensation = "" }, "", ""},
		{"kind empty", func(p *listings.CreateParams) { p.Kind = "" }, fKind, msgKind},
		{"kind unknown", func(p *listings.CreateParams) { p.Kind = "job" }, fKind, msgKind},
		{"title short", func(p *listings.CreateParams) { p.Title = "ab" }, fTitle, msgTitle},
		{"title short after trim", func(p *listings.CreateParams) { p.Title = "  ab  " }, fTitle, msgTitle},
		{"title 3 runes cyrillic", func(p *listings.CreateParams) { p.Title = "абв" }, "", ""},
		{"title 120 runes", func(p *listings.CreateParams) { p.Title = strings.Repeat("я", 120) }, "", ""},
		{"title 121 runes", func(p *listings.CreateParams) { p.Title = strings.Repeat("я", 121) }, fTitle, msgTitle},
		{"description short", func(p *listings.CreateParams) { p.Description = "коротко" }, fDescription, "Описание: от 10 до 5000 символов"},
		{"description long", func(p *listings.CreateParams) { p.Description = strings.Repeat("a", 5001) }, fDescription, "Описание: от 10 до 5000 символов"},
		{"looking_for empty", func(p *listings.CreateParams) { p.LookingFor = "   " }, fLookingFor, "Кого ищете: от 1 до 1000 символов"},
		{"looking_for long", func(p *listings.CreateParams) { p.LookingFor = strings.Repeat("a", 1001) }, fLookingFor, "Кого ищете: от 1 до 1000 символов"},
		{"tags 11", func(p *listings.CreateParams) { p.Tags = manyTags(11) }, fTags, "Теги: не больше 10"},
		{"tags 10 plus empties", func(p *listings.CreateParams) { p.Tags = append(manyTags(10), "", "  ") }, "", ""},
		{"tag too long", func(p *listings.CreateParams) { p.Tags = []string{strings.Repeat("т", 41)} }, fTags, "Теги: каждый не длиннее 40 символов"},
		{"tags duplicate case-insensitive", func(p *listings.CreateParams) { p.Tags = []string{"Go", " go "} }, fTags, "Теги: без повторов"},
		{"tags duplicate cyrillic case", func(p *listings.CreateParams) { p.Tags = []string{"Дизайн", "дизайн"} }, fTags, "Теги: без повторов"},
		{"startup without stage", func(p *listings.CreateParams) { p.Stage = "" }, fStage, "Стадия: укажите стадию"},
		{"startup bad stage", func(p *listings.CreateParams) { p.Stage = "seed" }, fStage, "Стадия: недопустимое значение"},
		{"vacancy with stage", func(p *listings.CreateParams) { *p = validVacancy(); p.Stage = listings.StageIdea }, fStage, "Стадия: только для стартапа"},
		{"startup with compensation", func(p *listings.CreateParams) { p.Compensation = "зарплата" }, fCompensation, "Условия: только для вакансии"},
		{"startup with blank compensation", func(p *listings.CreateParams) { p.Compensation = "  " }, "", ""},
		{"vacancy compensation long", func(p *listings.CreateParams) { *p = validVacancy(); p.Compensation = strings.Repeat("a", 121) }, fCompensation, "Условия: не больше 120 символов"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := validStartup()
			tt.mutate(&p)
			got, err := domain.NormalizeCreate(p)
			if tt.field != "" {
				wantValidation(t, err, tt.field, tt.msg)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Tags == nil {
				t.Fatal("Tags must not be nil")
			}
		})
	}
}

func TestNormalizeCreateTrims(t *testing.T) {
	t.Parallel()
	p := validVacancy()
	p.Title = "  Ищем дизайнера \n"
	p.Description = "\tОписание вакансии  "
	p.LookingFor = " UI/UX "
	p.Compensation = "  Оплата  "
	p.Tags = []string{" Figma ", "", "  ", "UX"}
	got, err := domain.NormalizeCreate(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Ищем дизайнера" || got.Description != "Описание вакансии" ||
		got.LookingFor != "UI/UX" || got.Compensation != "Оплата" {
		t.Fatalf("not trimmed: %+v", got)
	}
	if !reflect.DeepEqual(got.Tags, []string{"Figma", "UX"}) {
		t.Fatalf("Tags = %q", got.Tags)
	}
	if got.AuthorID != p.AuthorID || got.Kind != p.Kind {
		t.Fatal("AuthorID/Kind changed")
	}
}

func TestNormalizeTagsNil(t *testing.T) {
	t.Parallel()
	got, err := domain.NormalizeTags(nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("NormalizeTags(nil) = %#v, %v", got, err)
	}
}

func TestNormalizeUpdate(t *testing.T) {
	t.Parallel()
	base := listings.UpdateParams{
		Title: "Название", Description: "Достаточно длинное описание", LookingFor: "Кто-то",
	}
	tests := []struct {
		name   string
		kind   listings.Kind
		mutate func(p *listings.UpdateParams)
		field  string
		msg    string
	}{
		{"startup ok", listings.KindStartup, func(p *listings.UpdateParams) { p.Stage = listings.StageIdea }, "", ""},
		{"startup no stage", listings.KindStartup, func(*listings.UpdateParams) {}, fStage, "Стадия: укажите стадию"},
		{"vacancy ok", listings.KindVacancy, func(p *listings.UpdateParams) { p.Compensation = "Доля" }, "", ""},
		{"vacancy stage", listings.KindVacancy, func(p *listings.UpdateParams) { p.Stage = listings.StageMVP }, fStage, "Стадия: только для стартапа"},
		{"startup compensation", listings.KindStartup, func(p *listings.UpdateParams) {
			p.Stage = listings.StageIdea
			p.Compensation = "x"
		}, fCompensation, "Условия: только для вакансии"},
		{"title", listings.KindVacancy, func(p *listings.UpdateParams) { p.Title = "" }, fTitle, msgTitle},
		{"bad kind", "", func(*listings.UpdateParams) {}, fKind, msgKind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := base
			tt.mutate(&p)
			got, err := domain.NormalizeUpdate(tt.kind, p)
			if tt.field != "" {
				wantValidation(t, err, tt.field, tt.msg)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Tags == nil {
				t.Fatal("Tags must not be nil")
			}
		})
	}
}

func TestNormalizeFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		in    listings.ListFilter
		want  listings.ListFilter
		field string
		msg   string
	}{
		{"defaults", listings.ListFilter{}, listings.ListFilter{Page: 1, Limit: 20}, "", ""},
		{"explicit", listings.ListFilter{Kind: listings.KindVacancy, Query: " go ", Page: 3, Limit: 50},
			listings.ListFilter{Kind: listings.KindVacancy, Query: "go", Page: 3, Limit: 50}, "", ""},
		{"limit 1", listings.ListFilter{Limit: 1}, listings.ListFilter{Page: 1, Limit: 1}, "", ""},
		{"page negative", listings.ListFilter{Page: -1}, listings.ListFilter{}, fPage, "Страница: не меньше 1"},
		{"limit 51", listings.ListFilter{Limit: 51}, listings.ListFilter{}, fLimit, "Размер страницы: от 1 до 50"},
		{"limit negative", listings.ListFilter{Limit: -5}, listings.ListFilter{}, fLimit, "Размер страницы: от 1 до 50"},
		{"query 100", listings.ListFilter{Query: strings.Repeat("я", 100)},
			listings.ListFilter{Query: strings.Repeat("я", 100), Page: 1, Limit: 20}, "", ""},
		{"query 101", listings.ListFilter{Query: strings.Repeat("я", 101)}, listings.ListFilter{}, fQuery, "Поиск: не больше 100 символов"},
		{"bad kind", listings.ListFilter{Kind: "job"}, listings.ListFilter{}, fKind, msgKind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NormalizeFilter(tt.in)
			if tt.field != "" {
				wantValidation(t, err, tt.field, tt.msg)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestOffset(t *testing.T) {
	t.Parallel()
	if got := domain.Offset(listings.ListFilter{Page: 1000, Limit: 50}); got != 49950 {
		t.Fatalf("Offset = %d", got)
	}
	if got := domain.Offset(listings.ListFilter{Page: 1, Limit: 20}); got != 0 {
		t.Fatalf("Offset = %d", got)
	}
}

func TestValidateStatusFilter(t *testing.T) {
	t.Parallel()
	for _, s := range []listings.Status{"", listings.StatusActive, listings.StatusClosed} {
		if err := domain.ValidateStatusFilter(s); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
	}
	wantValidation(t, domain.ValidateStatusFilter("draft"), "status", "Статус: недопустимое значение")
}

func TestTransitions(t *testing.T) {
	t.Parallel()
	author := uuid.New()
	active := &listings.Listing{AuthorID: author, Status: listings.StatusActive}
	closed := &listings.Listing{AuthorID: author, Status: listings.StatusClosed}

	if err := domain.CheckAuthor(active, author); err != nil {
		t.Fatal(err)
	}
	err := domain.CheckAuthor(active, uuid.New())
	if apperr.KindOf(err) != apperr.KindForbidden || err.(*apperr.Error).Msg != "Недостаточно прав" {
		t.Fatalf("CheckAuthor(other) = %v", err)
	}

	if err := domain.CanEdit(active); err != nil {
		t.Fatal(err)
	}
	err = domain.CanEdit(closed)
	if apperr.KindOf(err) != apperr.KindPrecondition || err.(*apperr.Error).Msg != "Публикация закрыта" {
		t.Fatalf("CanEdit(closed) = %v", err)
	}

	if !domain.NeedsClose(active) || domain.NeedsClose(closed) {
		t.Fatal("NeedsClose")
	}
}

func manyTags(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "tag" + strings.Repeat("x", i)
	}
	return out
}
