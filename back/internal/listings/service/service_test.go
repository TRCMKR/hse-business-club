package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/listings/domain"
	"github.com/trcmkr/hse-business-club/back/internal/listings/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/listings/service"
	"github.com/trcmkr/hse-business-club/back/internal/listings/service/mocks"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

var _ service.Repository = (*postgres.Repository)(nil)

const (
	msgNotFound  = "Публикация не найдена"
	msgForbidden = "Недостаточно прав"
	msgClosed    = "Публикация закрыта"
	msgStage     = "Стадия: укажите стадию"
)

var created = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func startupParams(author uuid.UUID) listings.CreateParams {
	return listings.CreateParams{
		AuthorID: author, Kind: listings.KindStartup, Title: "  Стартап  ",
		Description: "Описание стартапа", LookingFor: "Разработчик",
		Tags: []string{" Go ", ""}, Stage: listings.StageIdea,
	}
}

func updateParams() listings.UpdateParams {
	return listings.UpdateParams{
		Title: "  Новое название ", Description: "Новое описание", LookingFor: "Дизайнер",
		Tags: []string{"Figma", " "}, Stage: listings.StageMVP,
	}
}

// listing returns a stored startup of author in status st.
func listing(author uuid.UUID, st listings.Status) *listings.Listing {
	l := &listings.Listing{
		ID: uuid.New(), AuthorID: author, Kind: listings.KindStartup, Title: "Стартап",
		Description: "Описание стартапа", LookingFor: "Разработчик", Tags: []string{"Go"},
		Stage: listings.StageIdea, Status: st, CreatedAt: created, UpdatedAt: created,
	}
	if st == listings.StatusClosed {
		at := created.Add(time.Hour)
		l.ClosedAt = &at
	}
	return l
}

func closedCopy(l *listings.Listing) *listings.Listing {
	c := *l
	at := created.Add(time.Hour)
	c.Status, c.ClosedAt, c.UpdatedAt = listings.StatusClosed, &at, at
	return &c
}

func wantErr(t *testing.T, err error, kind apperr.Kind, msg string) {
	t.Helper()
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	require.Equal(t, kind, ae.Kind, "err = %v", err)
	if msg != "" {
		require.Equal(t, msg, ae.Msg)
	}
}

func setup(t *testing.T) (*service.Service, *mocks.MockRepository) {
	t.Helper()
	repo := mocks.NewMockRepository(t)
	return service.New(repo), repo
}

func TestCreate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("normalized params reach the repo", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		want := startupParams(author)
		want.Title, want.Tags = "Стартап", []string{"Go"}
		stored := listing(author, listings.StatusActive)
		repo.EXPECT().Create(mock.Anything, want).Return(stored, nil).Once()

		l, err := svc.Create(ctx, startupParams(author))
		require.NoError(t, err)
		assert.Same(t, stored, l)
	})
	t.Run("invalid input does not reach the repo", func(t *testing.T) {
		t.Parallel()
		svc, _ := setup(t)
		p := startupParams(uuid.New())
		p.Stage = ""
		_, err := svc.Create(ctx, p)
		wantErr(t, err, apperr.KindValidation, msgStage)
	})
}

func TestGet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, repo := setup(t)

	for _, l := range []*listings.Listing{listing(uuid.New(), listings.StatusActive), listing(uuid.New(), listings.StatusClosed)} {
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		got, err := svc.Get(ctx, l.ID)
		require.NoError(t, err)
		assert.Same(t, l, got, "a %s listing is returned too", l.Status)
	}

	missing := uuid.New()
	repo.EXPECT().GetByID(mock.Anything, missing).Return(nil, domain.ErrNotFound()).Once()
	_, err := svc.Get(ctx, missing)
	wantErr(t, err, apperr.KindNotFound, msgNotFound)
}

func TestGetMany(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, repo := setup(t)
	l := listing(uuid.New(), listings.StatusActive)
	ids := []uuid.UUID{l.ID, uuid.New()}
	repo.EXPECT().GetMany(mock.Anything, ids).Return(map[uuid.UUID]*listings.Listing{l.ID: l}, nil).Once()

	got, err := svc.GetMany(ctx, ids)
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]*listings.Listing{l.ID: l}, got)

	empty, err := svc.GetMany(ctx, nil) // no repo call
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestListActive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("defaults and trimmed query", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		items := []*listings.Listing{listing(uuid.New(), listings.StatusActive), listing(uuid.New(), listings.StatusActive)}
		repo.EXPECT().ListActive(mock.Anything, listings.ListFilter{Query: "go", Page: 1, Limit: 20}).Return(items, 2, nil).Once()

		page, err := svc.ListActive(ctx, listings.ListFilter{Query: "  go  "})
		require.NoError(t, err)
		assert.Equal(t, &listings.Page{Items: items, Total: 2, Page: 1, Limit: 20}, page)
	})
	t.Run("kind filter and explicit page past the end", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		f := listings.ListFilter{Kind: listings.KindVacancy, Page: 1000, Limit: 50}
		repo.EXPECT().ListActive(mock.Anything, f).Return([]*listings.Listing{}, 2, nil).Once()

		page, err := svc.ListActive(ctx, f)
		require.NoError(t, err)
		assert.Empty(t, page.Items)
		assert.Equal(t, 2, page.Total)
		assert.Equal(t, 1000, page.Page)
		assert.Equal(t, 50, page.Limit)
	})
	t.Run("repo failure", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		repo.EXPECT().ListActive(mock.Anything, mock.Anything).
			Return(nil, 0, apperr.Wrap(apperr.KindUnknown, "Внутренняя ошибка", errors.New("db down"))).Once()
		_, err := svc.ListActive(ctx, listings.ListFilter{})
		wantErr(t, err, apperr.KindUnknown, "")
	})
	t.Run("invalid filter does not reach the repo", func(t *testing.T) {
		t.Parallel()
		svc, _ := setup(t)
		for f, msg := range map[listings.ListFilter]string{
			{Limit: 51}:   "Размер страницы: от 1 до 50",
			{Page: -1}:    "Страница: не меньше 1",
			{Kind: "job"}: "Вид публикации: выберите стартап или вакансию",
		} {
			_, err := svc.ListActive(ctx, f)
			wantErr(t, err, apperr.KindValidation, msg)
		}
	})
}

func TestListByAuthor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, repo := setup(t)
	author := uuid.New()
	all := []*listings.Listing{listing(author, listings.StatusClosed)}

	repo.EXPECT().ListByAuthor(mock.Anything, author, listings.Status("")).Return(all, nil).Once()
	got, err := svc.ListByAuthor(ctx, author, "")
	require.NoError(t, err)
	assert.Equal(t, all, got)

	repo.EXPECT().ListByAuthor(mock.Anything, author, listings.StatusActive).Return([]*listings.Listing{}, nil).Once()
	got, err = svc.ListByAuthor(ctx, author, listings.StatusActive)
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = svc.ListByAuthor(ctx, author, "draft")
	wantErr(t, err, apperr.KindValidation, "Статус: недопустимое значение")
}

func TestUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("author edits active", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusActive)
		want := updateParams()
		want.Title, want.Tags = "Новое название", []string{"Figma"}
		updated := *l
		updated.Title, updated.Stage = want.Title, want.Stage
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		repo.EXPECT().Update(mock.Anything, l.ID, want).Return(&updated, nil).Once()

		got, err := svc.Update(ctx, l.ID, author, updateParams())
		require.NoError(t, err)
		assert.Same(t, &updated, got)
		assert.Equal(t, listings.KindStartup, got.Kind)
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		id := uuid.New()
		repo.EXPECT().GetByID(mock.Anything, id).Return(nil, domain.ErrNotFound()).Once()
		_, err := svc.Update(ctx, id, uuid.New(), updateParams())
		wantErr(t, err, apperr.KindNotFound, msgNotFound)
	})
	t.Run("non-author forbidden", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		l := listing(uuid.New(), listings.StatusActive)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once() // no Update
		_, err := svc.Update(ctx, l.ID, uuid.New(), updateParams())
		wantErr(t, err, apperr.KindForbidden, msgForbidden)
	})
	t.Run("non-author on closed is forbidden, not precondition", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		l := listing(uuid.New(), listings.StatusClosed)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		_, err := svc.Update(ctx, l.ID, uuid.New(), updateParams())
		wantErr(t, err, apperr.KindForbidden, msgForbidden)
	})
	t.Run("closed", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusClosed)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		_, err := svc.Update(ctx, l.ID, author, updateParams())
		wantErr(t, err, apperr.KindPrecondition, msgClosed)
	})
	t.Run("closed and invalid: status is checked before input", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusClosed)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		_, err := svc.Update(ctx, l.ID, author, listings.UpdateParams{})
		wantErr(t, err, apperr.KindPrecondition, msgClosed)
	})
	t.Run("validation uses stored kind", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusActive)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		p := updateParams()
		p.Stage, p.Compensation = "", "Оплата"
		_, err := svc.Update(ctx, l.ID, author, p)
		wantErr(t, err, apperr.KindValidation, msgStage)
	})
	t.Run("closed concurrently", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusActive)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		repo.EXPECT().Update(mock.Anything, l.ID, mock.Anything).Return(nil, domain.ErrNotActive).Once()
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(closedCopy(l), nil).Once()
		_, err := svc.Update(ctx, l.ID, author, updateParams())
		wantErr(t, err, apperr.KindPrecondition, msgClosed)
	})
	t.Run("deleted concurrently", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusActive)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		repo.EXPECT().Update(mock.Anything, l.ID, mock.Anything).Return(nil, domain.ErrNotActive).Once()
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(nil, domain.ErrNotFound()).Once()
		_, err := svc.Update(ctx, l.ID, author, updateParams())
		wantErr(t, err, apperr.KindNotFound, msgNotFound)
	})
	t.Run("repo failure", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusActive)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		repo.EXPECT().Update(mock.Anything, l.ID, mock.Anything).
			Return(nil, apperr.Wrap(apperr.KindUnknown, "Внутренняя ошибка", errors.New("db down"))).Once()
		_, err := svc.Update(ctx, l.ID, author, updateParams())
		wantErr(t, err, apperr.KindUnknown, "")
	})
}

func TestClose(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("author closes active", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusActive)
		closed := closedCopy(l)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		repo.EXPECT().Close(mock.Anything, l.ID).Return(closed, nil).Once()
		got, err := svc.Close(ctx, l.ID, author)
		require.NoError(t, err)
		assert.Same(t, closed, got)
	})
	t.Run("closing a closed one is idempotent and does not write", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusClosed)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once() // no Close
		got, err := svc.Close(ctx, l.ID, author)
		require.NoError(t, err)
		assert.Same(t, l, got)
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		id := uuid.New()
		repo.EXPECT().GetByID(mock.Anything, id).Return(nil, domain.ErrNotFound()).Once()
		_, err := svc.Close(ctx, id, uuid.New())
		wantErr(t, err, apperr.KindNotFound, msgNotFound)
	})
	for _, st := range []listings.Status{listings.StatusActive, listings.StatusClosed} {
		t.Run("non-author on "+string(st)+" forbidden", func(t *testing.T) {
			t.Parallel()
			svc, repo := setup(t)
			l := listing(uuid.New(), st)
			repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
			_, err := svc.Close(ctx, l.ID, uuid.New())
			wantErr(t, err, apperr.KindForbidden, msgForbidden)
		})
	}
	t.Run("closed concurrently: still idempotent", func(t *testing.T) {
		t.Parallel()
		svc, repo := setup(t)
		author := uuid.New()
		l := listing(author, listings.StatusActive)
		closed := closedCopy(l)
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(l, nil).Once()
		repo.EXPECT().Close(mock.Anything, l.ID).Return(nil, domain.ErrNotActive).Once()
		repo.EXPECT().GetByID(mock.Anything, l.ID).Return(closed, nil).Once()
		got, err := svc.Close(ctx, l.ID, author)
		require.NoError(t, err)
		assert.Same(t, closed, got)
	})
}
