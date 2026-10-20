package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/applications/domain"
	appmocks "github.com/trcmkr/hse-business-club/back/internal/applications/mocks"
	"github.com/trcmkr/hse-business-club/back/internal/applications/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/applications/service"
	"github.com/trcmkr/hse-business-club/back/internal/applications/service/mocks"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

var _ service.Repository = (*postgres.Repo)(nil)

var decidedNow = time.Date(2026, 10, 20, 9, 30, 0, 123456789, time.FixedZone("MSK", 3*3600))

// SPEC §2.3 messages, spelled out to check the user-facing text.
const (
	testMessage  = "Хочу в команду"
	msgNotFound  = "Отклик не найден"
	msgForbidden = "Недостаточно прав"
)

type env struct {
	svc      *service.Service
	repo     *mocks.MockRepository
	listings *appmocks.MockListingReader
	author   uuid.UUID
	open     uuid.UUID // active listing of author
	closed   uuid.UUID // closed listing of author
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{
		repo: mocks.NewMockRepository(t), listings: appmocks.NewMockListingReader(t),
		author: uuid.New(), open: uuid.New(), closed: uuid.New(),
	}
	e.svc = service.New(e.repo, e.listings, service.WithClock(func() time.Time { return decidedNow }))
	return e
}

func (e *env) expectListing(id uuid.UUID, isOpen bool) {
	e.listings.EXPECT().GetListing(mock.Anything, id).
		Return(applications.ListingInfo{ID: id, AuthorID: e.author, IsOpen: isOpen}, nil).Once()
}

// application returns a stored application of applicant to e.open in status st.
func (e *env) application(applicant uuid.UUID, st applications.Status) *applications.Application {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return &applications.Application{
		ID: uuid.New(), ListingID: e.open, ListingAuthorID: e.author, ApplicantID: applicant,
		Message: testMessage, Status: st, CreatedAt: at, UpdatedAt: at,
	}
}

// updateResult records what fn passed to Repository.Update decided.
type updateResult struct {
	called bool
	change *domain.StatusChange
	err    error
}

// expectUpdate mocks the transactional Repository.Update like postgres.Repo: fn is called
// with a copy of cur, a non-nil change is applied, a nil change returns cur unchanged.
func (e *env) expectUpdate(cur *applications.Application) *updateResult {
	res := &updateResult{}
	e.repo.EXPECT().Update(mock.Anything, cur.ID, mock.Anything).RunAndReturn(
		func(_ context.Context, _ uuid.UUID, fn func(*applications.Application) (*domain.StatusChange, error)) (*applications.Application, error) {
			c := *cur
			res.called = true
			res.change, res.err = fn(&c)
			if res.err != nil {
				return nil, res.err
			}
			out := *cur
			if res.change != nil {
				out.Status, out.DecidedAt = res.change.Status, res.change.DecidedAt
			}
			return &out, nil
		}).Once()
	return res
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

func TestApply(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a pending application with the listing author copied", func(t *testing.T) {
		e := newEnv(t)
		me := uuid.New()
		e.expectListing(e.open, true)
		created := e.application(me, applications.StatusPending)
		e.repo.EXPECT().Create(mock.Anything, &applications.Application{
			ListingID: e.open, ListingAuthorID: e.author, ApplicantID: me, Message: testMessage,
		}).Return(created, nil).Once()

		a, err := e.svc.Apply(ctx, e.open, me, "  Хочу в команду \n")
		require.NoError(t, err)
		assert.Same(t, created, a)
	})

	t.Run("again while pending/accepted/declined → conflict", func(t *testing.T) {
		// The live-application uniqueness is the repository's (partial unique index,
		// integration tests); the service passes its conflict through.
		e := newEnv(t)
		e.expectListing(e.open, true)
		e.repo.EXPECT().Create(mock.Anything, mock.Anything).
			Return(nil, apperr.New(apperr.KindConflict, domain.MsgAlreadyApplied)).Once()
		_, err := e.svc.Apply(ctx, e.open, uuid.New(), "ещё раз")
		wantErr(t, err, apperr.KindConflict, "Вы уже откликались на эту публикацию")
	})

	t.Run("own listing", func(t *testing.T) {
		e := newEnv(t)
		e.expectListing(e.open, true)
		_, err := e.svc.Apply(ctx, e.open, e.author, "сам себе")
		wantErr(t, err, apperr.KindPrecondition, "Нельзя откликнуться на свою публикацию")
	})

	t.Run("closed listing", func(t *testing.T) {
		e := newEnv(t)
		e.expectListing(e.closed, false)
		_, err := e.svc.Apply(ctx, e.closed, uuid.New(), "поздно")
		wantErr(t, err, apperr.KindPrecondition, "Публикация закрыта")
	})

	t.Run("missing listing", func(t *testing.T) {
		e := newEnv(t)
		id := uuid.New()
		e.listings.EXPECT().GetListing(mock.Anything, id).
			Return(applications.ListingInfo{}, apperr.New(apperr.KindNotFound, "Публикация не найдена")).Once()
		_, err := e.svc.Apply(ctx, id, uuid.New(), "куда")
		wantErr(t, err, apperr.KindNotFound, "Публикация не найдена")
	})

	t.Run("message validation before any lookup", func(t *testing.T) {
		e := newEnv(t) // no expectations: neither listings nor repo may be called
		for _, m := range []string{"", "   ", strings.Repeat("ы", 2001)} {
			_, err := e.svc.Apply(ctx, e.open, uuid.New(), m)
			wantErr(t, err, apperr.KindValidation, "Сообщение: от 1 до 2000 символов")
		}
	})
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	me := uuid.New()
	a := e.application(me, applications.StatusPending)
	e.repo.EXPECT().GetByID(mock.Anything, a.ID).Return(a, nil).Times(3)

	for name, actor := range map[string]uuid.UUID{"applicant": me, "author": e.author} {
		got, err := e.svc.Get(ctx, a.ID, actor)
		require.NoError(t, err, name)
		assert.Same(t, a, got, name)
	}
	// stranger: existence is not disclosed
	_, err := e.svc.Get(ctx, a.ID, uuid.New())
	wantErr(t, err, apperr.KindNotFound, msgNotFound)

	missing := uuid.New()
	e.repo.EXPECT().GetByID(mock.Anything, missing).Return(nil, apperr.New(apperr.KindNotFound, domain.MsgNotFound)).Once()
	_, err = e.svc.Get(ctx, missing, me)
	wantErr(t, err, apperr.KindNotFound, msgNotFound)
}

// TestTransitions covers accept/decline/withdraw: who may do what (SPEC §2.3, checked
// before the status), the status machine, decided_at and idempotent repeats.
func TestTransitions(t *testing.T) {
	ctx := context.Background()
	const (
		author    = "author"
		applicant = "applicant"
		stranger  = "stranger"
	)
	type action func(s *service.Service) func(context.Context, uuid.UUID, uuid.UUID) (*applications.Application, error)
	accept := func(s *service.Service) func(context.Context, uuid.UUID, uuid.UUID) (*applications.Application, error) {
		return s.Accept
	}
	decline := func(s *service.Service) func(context.Context, uuid.UUID, uuid.UUID) (*applications.Application, error) {
		return s.Decline
	}
	withdraw := func(s *service.Service) func(context.Context, uuid.UUID, uuid.UUID) (*applications.Application, error) {
		return s.Withdraw
	}
	const (
		pending   = applications.StatusPending
		accepted  = applications.StatusAccepted
		declined  = applications.StatusDeclined
		withdrawn = applications.StatusWithdrawn
	)

	for _, tc := range []struct {
		name    string
		do      action
		actor   string
		from    applications.Status
		to      applications.Status // "" — no write (noop or error)
		decided bool
		kind    apperr.Kind // 0 — success
		msg     string
	}{
		{name: "accept pending", do: accept, actor: author, from: pending, to: accepted, decided: true},
		{name: "decline pending", do: decline, actor: author, from: pending, to: declined, decided: true},
		{name: "withdraw pending", do: withdraw, actor: applicant, from: pending, to: withdrawn},

		{name: "repeat accept is a noop", do: accept, actor: author, from: accepted},
		{name: "repeat decline is a noop", do: decline, actor: author, from: declined},
		{name: "repeat withdraw is a noop", do: withdraw, actor: applicant, from: withdrawn},

		{name: "decline accepted", do: decline, actor: author, from: accepted, kind: apperr.KindPrecondition, msg: domain.MsgAlreadyDecided},
		{name: "accept declined", do: accept, actor: author, from: declined, kind: apperr.KindPrecondition, msg: domain.MsgAlreadyDecided},
		{name: "accept withdrawn", do: accept, actor: author, from: withdrawn, kind: apperr.KindPrecondition, msg: domain.MsgAlreadyDecided},
		{name: "decline withdrawn", do: decline, actor: author, from: withdrawn, kind: apperr.KindPrecondition, msg: domain.MsgAlreadyDecided},
		{name: "withdraw accepted", do: withdraw, actor: applicant, from: accepted, kind: apperr.KindPrecondition, msg: domain.MsgAlreadyDecided},
		{name: "withdraw declined", do: withdraw, actor: applicant, from: declined, kind: apperr.KindPrecondition, msg: domain.MsgAlreadyDecided},

		{name: "stranger accepts", do: accept, actor: stranger, from: pending, kind: apperr.KindNotFound, msg: msgNotFound},
		{name: "stranger declines", do: decline, actor: stranger, from: pending, kind: apperr.KindNotFound, msg: msgNotFound},
		{name: "stranger withdraws", do: withdraw, actor: stranger, from: pending, kind: apperr.KindNotFound, msg: msgNotFound},

		{name: "applicant accepts", do: accept, actor: applicant, from: pending, kind: apperr.KindForbidden, msg: msgForbidden},
		{name: "applicant declines", do: decline, actor: applicant, from: pending, kind: apperr.KindForbidden, msg: msgForbidden},
		{name: "author withdraws", do: withdraw, actor: author, from: pending, kind: apperr.KindForbidden, msg: msgForbidden},
		// rights are checked before the status transition
		{name: "applicant accepts accepted", do: accept, actor: applicant, from: accepted, kind: apperr.KindForbidden, msg: msgForbidden},
		{name: "author withdraws declined", do: withdraw, actor: author, from: declined, kind: apperr.KindForbidden, msg: msgForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			me := uuid.New()
			actor := map[string]uuid.UUID{author: e.author, applicant: me, stranger: uuid.New()}[tc.actor]
			cur := e.application(me, tc.from)
			res := e.expectUpdate(cur)

			got, err := tc.do(e.svc)(ctx, cur.ID, actor)
			require.True(t, res.called)
			if tc.kind != 0 {
				wantErr(t, err, tc.kind, tc.msg)
				assert.Nil(t, res.change)
				return
			}
			require.NoError(t, err)
			if tc.to == "" {
				assert.Nil(t, res.change, "a repeat must not write")
				assert.Equal(t, cur, got, "a repeat returns the current state")
				return
			}
			require.NotNil(t, res.change)
			assert.Equal(t, tc.to, res.change.Status)
			assert.Equal(t, tc.to, got.Status)
			if !tc.decided {
				assert.Nil(t, res.change.DecidedAt)
				return
			}
			require.NotNil(t, res.change.DecidedAt)
			assert.True(t, res.change.DecidedAt.Equal(decidedNow.UTC().Truncate(time.Microsecond)), "decided_at = %s", res.change.DecidedAt)
			assert.Equal(t, time.UTC, res.change.DecidedAt.Location())
		})
	}

	t.Run("missing id", func(t *testing.T) {
		e := newEnv(t)
		id := uuid.New()
		for _, do := range []action{accept, decline, withdraw} {
			e.repo.EXPECT().Update(mock.Anything, id, mock.Anything).
				Return(nil, apperr.New(apperr.KindNotFound, domain.MsgNotFound)).Once()
			_, err := do(e.svc)(ctx, id, e.author)
			wantErr(t, err, apperr.KindNotFound, msgNotFound)
		}
	})
}

func TestListByListing(t *testing.T) {
	ctx := context.Background()

	t.Run("author gets the repository list as is", func(t *testing.T) {
		e := newEnv(t)
		e.expectListing(e.open, true)
		list := []*applications.Application{e.application(uuid.New(), applications.StatusAccepted)}
		e.repo.EXPECT().ListByListing(mock.Anything, e.open, applications.StatusAccepted).Return(list, nil).Once()
		got, err := e.svc.ListByListing(ctx, e.open, e.author, applications.StatusAccepted)
		require.NoError(t, err)
		assert.Equal(t, list, got)
	})

	t.Run("zero applications: author gets an empty list", func(t *testing.T) {
		// authorship is checked through the port, not through the (absent) rows
		e := newEnv(t)
		e.expectListing(e.open, true)
		e.repo.EXPECT().ListByListing(mock.Anything, e.open, applications.Status("")).
			Return([]*applications.Application{}, nil).Once()
		got, err := e.svc.ListByListing(ctx, e.open, e.author, "")
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("closed listing: author still sees", func(t *testing.T) {
		e := newEnv(t)
		e.expectListing(e.closed, false)
		e.repo.EXPECT().ListByListing(mock.Anything, e.closed, applications.Status("")).Return(nil, nil).Once()
		_, err := e.svc.ListByListing(ctx, e.closed, e.author, "")
		require.NoError(t, err)
	})

	t.Run("not the author (incl. an applicant) → 403", func(t *testing.T) {
		e := newEnv(t)
		e.expectListing(e.open, true)
		_, err := e.svc.ListByListing(ctx, e.open, uuid.New(), "")
		wantErr(t, err, apperr.KindForbidden, msgForbidden)
	})

	t.Run("missing listing", func(t *testing.T) {
		e := newEnv(t)
		id := uuid.New()
		e.listings.EXPECT().GetListing(mock.Anything, id).
			Return(applications.ListingInfo{}, apperr.New(apperr.KindNotFound, "Публикация не найдена")).Once()
		_, err := e.svc.ListByListing(ctx, id, e.author, "")
		wantErr(t, err, apperr.KindNotFound, "Публикация не найдена")
	})

	t.Run("bad status filter before any lookup", func(t *testing.T) {
		e := newEnv(t)
		_, err := e.svc.ListByListing(ctx, e.open, e.author, "nope")
		wantErr(t, err, apperr.KindValidation, domain.MsgStatusUnknown)
	})
}

func TestListByApplicant(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	me := uuid.New()
	list := []*applications.Application{e.application(me, applications.StatusWithdrawn), e.application(me, applications.StatusPending)}
	for _, st := range []applications.Status{"", applications.StatusPending} {
		e.repo.EXPECT().ListByApplicant(mock.Anything, me, st).Return(list, nil).Once()
		got, err := e.svc.ListByApplicant(ctx, me, st)
		require.NoError(t, err)
		assert.Equal(t, list, got, "the repository order (newest first) is kept")
	}
	_, err := e.svc.ListByApplicant(ctx, me, "bogus")
	wantErr(t, err, apperr.KindValidation, domain.MsgStatusUnknown)
}

func TestListReceived(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	list := []*applications.Application{e.application(uuid.New(), applications.StatusPending), e.application(uuid.New(), applications.StatusWithdrawn)}
	for _, st := range []applications.Status{"", applications.StatusDeclined} {
		e.repo.EXPECT().ListByListingAuthor(mock.Anything, e.author, st).Return(list, nil).Once()
		got, err := e.svc.ListReceived(ctx, e.author, st)
		require.NoError(t, err)
		assert.Equal(t, list, got, "the repository order (pending first) is kept")
	}
	_, err := e.svc.ListReceived(ctx, e.author, "bogus")
	wantErr(t, err, apperr.KindValidation, domain.MsgStatusUnknown)
}

func TestBatches(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	me, other, empty := uuid.New(), uuid.New(), uuid.New()
	ids := []uuid.UUID{e.open, other, empty}

	mine := map[uuid.UUID]*applications.Application{e.open: e.application(me, applications.StatusPending)}
	e.repo.EXPECT().GetMineForListings(mock.Anything, me, ids).Return(mine, nil).Once()
	m, err := e.svc.GetMineForListings(ctx, me, ids)
	require.NoError(t, err)
	assert.Equal(t, mine, m)

	stats := map[uuid.UUID]applications.Stats{e.open: {Pending: 2, Total: 2}, other: {}, empty: {}}
	e.repo.EXPECT().StatsForListings(mock.Anything, ids).Return(stats, nil).Once()
	st, err := e.svc.StatsForListings(ctx, ids)
	require.NoError(t, err)
	assert.Equal(t, stats, st)

	dbDown := apperr.Wrap(apperr.KindUnknown, "Внутренняя ошибка", errors.New("db down"))
	e.repo.EXPECT().StatsForListings(mock.Anything, ids).Return(nil, dbDown).Once()
	_, err = e.svc.StatsForListings(ctx, ids)
	wantErr(t, err, apperr.KindUnknown, "")
}
