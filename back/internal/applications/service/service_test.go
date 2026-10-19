package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/applications/service"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
)

var decidedNow = time.Date(2026, 10, 20, 9, 30, 0, 123456789, time.FixedZone("MSK", 3*3600))

type env struct {
	svc      *service.Service
	repo     *fakeRepo
	listings *fakeListings
	author   uuid.UUID
	open     uuid.UUID // active listing of author
	closed   uuid.UUID // closed listing of author
}

func newEnv() *env {
	e := &env{repo: newFakeRepo(), author: uuid.New(), open: uuid.New(), closed: uuid.New()}
	e.listings = &fakeListings{items: map[uuid.UUID]applications.ListingInfo{
		e.open:   {ID: e.open, AuthorID: e.author, IsOpen: true},
		e.closed: {ID: e.closed, AuthorID: e.author, IsOpen: false},
	}}
	e.svc = service.New(e.repo, e.listings, service.WithClock(func() time.Time { return decidedNow }))
	return e
}

func (e *env) apply(t *testing.T, applicant uuid.UUID) *applications.Application {
	t.Helper()
	a, err := e.svc.Apply(context.Background(), e.open, applicant, "Хочу в команду")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return a
}

func wantErr(t *testing.T, err error, kind apperr.Kind, msg string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind != kind {
		t.Fatalf("err = %v, want kind %v", err, kind)
	}
	if msg != "" && ae.Msg != msg {
		t.Fatalf("msg = %q, want %q", ae.Msg, msg)
	}
}

func TestApply(t *testing.T) {
	ctx := context.Background()
	e := newEnv()
	me := uuid.New()

	a, err := e.svc.Apply(ctx, e.open, me, "  Хочу в команду \n")
	if err != nil {
		t.Fatal(err)
	}
	if a.ListingID != e.open || a.ApplicantID != me || a.ListingAuthorID != e.author ||
		a.Status != applications.StatusPending || a.Message != "Хочу в команду" || a.DecidedAt != nil {
		t.Fatalf("applied = %+v", a)
	}

	t.Run("again while pending/accepted/declined → conflict", func(t *testing.T) {
		_, err := e.svc.Apply(ctx, e.open, me, "ещё раз")
		wantErr(t, err, apperr.KindConflict, "Вы уже откликались на эту публикацию")
		if _, err := e.svc.Decline(ctx, a.ID, e.author); err != nil {
			t.Fatal(err)
		}
		_, err = e.svc.Apply(ctx, e.open, me, "ещё раз")
		wantErr(t, err, apperr.KindConflict, "Вы уже откликались на эту публикацию")
	})

	t.Run("after withdraw → new record", func(t *testing.T) {
		other := uuid.New()
		first := e.apply(t, other)
		if _, err := e.svc.Withdraw(ctx, first.ID, other); err != nil {
			t.Fatal(err)
		}
		second := e.apply(t, other)
		if second.ID == first.ID || second.Status != applications.StatusPending {
			t.Fatalf("second = %+v", second)
		}
	})

	t.Run("own listing", func(t *testing.T) {
		_, err := e.svc.Apply(ctx, e.open, e.author, "сам себе")
		wantErr(t, err, apperr.KindPrecondition, "Нельзя откликнуться на свою публикацию")
	})

	t.Run("closed listing", func(t *testing.T) {
		_, err := e.svc.Apply(ctx, e.closed, uuid.New(), "поздно")
		wantErr(t, err, apperr.KindPrecondition, "Публикация закрыта")
	})

	t.Run("missing listing", func(t *testing.T) {
		_, err := e.svc.Apply(ctx, uuid.New(), uuid.New(), "куда")
		wantErr(t, err, apperr.KindNotFound, "")
	})

	t.Run("message validation before any lookup", func(t *testing.T) {
		calls, writes := e.listings.calls, e.repo.writes
		for _, m := range []string{"", "   ", strings.Repeat("ы", 2001)} {
			_, err := e.svc.Apply(ctx, e.open, uuid.New(), m)
			wantErr(t, err, apperr.KindValidation, "Сообщение: от 1 до 2000 символов")
		}
		if e.listings.calls != calls || e.repo.writes != writes {
			t.Fatal("invalid message must not reach listings or repo")
		}
	})
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	e := newEnv()
	me := uuid.New()
	a := e.apply(t, me)

	for name, actor := range map[string]uuid.UUID{"applicant": me, "author": e.author} {
		got, err := e.svc.Get(ctx, a.ID, actor)
		if err != nil || got.ID != a.ID {
			t.Fatalf("%s: %v %v", name, got, err)
		}
	}
	_, err := e.svc.Get(ctx, a.ID, uuid.New())
	wantErr(t, err, apperr.KindNotFound, "Отклик не найден")
	_, err = e.svc.Get(ctx, uuid.New(), me)
	wantErr(t, err, apperr.KindNotFound, "Отклик не найден")
}

func TestDecisions(t *testing.T) {
	ctx := context.Background()
	e := newEnv()

	t.Run("accept sets decided_at, repeat is a no-op", func(t *testing.T) {
		a := e.apply(t, uuid.New())
		got, err := e.svc.Accept(ctx, a.ID, e.author)
		if err != nil {
			t.Fatal(err)
		}
		wantDecided := decidedNow.UTC().Truncate(time.Microsecond)
		if got.Status != applications.StatusAccepted || got.DecidedAt == nil || !got.DecidedAt.Equal(wantDecided) ||
			got.DecidedAt.Location() != time.UTC {
			t.Fatalf("accepted = %+v", got)
		}
		writes := e.repo.writes
		again, err := e.svc.Accept(ctx, a.ID, e.author)
		if err != nil || *again != *got {
			t.Fatalf("repeat accept: %+v %v (want %+v)", again, err, got)
		}
		if e.repo.writes != writes {
			t.Fatal("no-op must not write")
		}
		_, err = e.svc.Decline(ctx, a.ID, e.author)
		wantErr(t, err, apperr.KindPrecondition, "Решение по отклику уже принято")
	})

	t.Run("decline, repeat is a no-op", func(t *testing.T) {
		a := e.apply(t, uuid.New())
		got, err := e.svc.Decline(ctx, a.ID, e.author)
		if err != nil || got.Status != applications.StatusDeclined || got.DecidedAt == nil {
			t.Fatalf("declined = %+v %v", got, err)
		}
		writes := e.repo.writes
		again, err := e.svc.Decline(ctx, a.ID, e.author)
		if err != nil || *again != *got || e.repo.writes != writes {
			t.Fatalf("repeat decline: %+v %v", again, err)
		}
		_, err = e.svc.Accept(ctx, a.ID, e.author)
		wantErr(t, err, apperr.KindPrecondition, "Решение по отклику уже принято")
	})

	t.Run("decisions on withdrawn", func(t *testing.T) {
		me := uuid.New()
		a := e.apply(t, me)
		if _, err := e.svc.Withdraw(ctx, a.ID, me); err != nil {
			t.Fatal(err)
		}
		_, err := e.svc.Accept(ctx, a.ID, e.author)
		wantErr(t, err, apperr.KindPrecondition, "Решение по отклику уже принято")
		_, err = e.svc.Decline(ctx, a.ID, e.author)
		wantErr(t, err, apperr.KindPrecondition, "Решение по отклику уже принято")
	})

	t.Run("only the listing author decides", func(t *testing.T) {
		me := uuid.New()
		a := e.apply(t, me)
		// stranger: existence is not disclosed
		_, err := e.svc.Accept(ctx, a.ID, uuid.New())
		wantErr(t, err, apperr.KindNotFound, "Отклик не найден")
		_, err = e.svc.Decline(ctx, a.ID, uuid.New())
		wantErr(t, err, apperr.KindNotFound, "Отклик не найден")
		// the applicant sees the application, so gets 403
		_, err = e.svc.Accept(ctx, a.ID, me)
		wantErr(t, err, apperr.KindForbidden, "Недостаточно прав")
		_, err = e.svc.Decline(ctx, a.ID, me)
		wantErr(t, err, apperr.KindForbidden, "Недостаточно прав")
		if got, _ := e.svc.Get(ctx, a.ID, me); got.Status != applications.StatusPending {
			t.Fatalf("status changed: %s", got.Status)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		_, err := e.svc.Accept(ctx, uuid.New(), e.author)
		wantErr(t, err, apperr.KindNotFound, "")
	})
}

func TestWithdraw(t *testing.T) {
	ctx := context.Background()
	e := newEnv()
	me := uuid.New()

	a := e.apply(t, me)
	_, err := e.svc.Withdraw(ctx, a.ID, uuid.New())
	wantErr(t, err, apperr.KindNotFound, "Отклик не найден")
	_, err = e.svc.Withdraw(ctx, a.ID, e.author)
	wantErr(t, err, apperr.KindForbidden, "Недостаточно прав")

	got, err := e.svc.Withdraw(ctx, a.ID, me)
	if err != nil || got.Status != applications.StatusWithdrawn || got.DecidedAt != nil {
		t.Fatalf("withdrawn = %+v %v", got, err)
	}
	writes := e.repo.writes
	again, err := e.svc.Withdraw(ctx, a.ID, me)
	if err != nil || *again != *got || e.repo.writes != writes {
		t.Fatalf("repeat withdraw: %+v %v", again, err)
	}

	for _, decide := range []func(context.Context, uuid.UUID, uuid.UUID) (*applications.Application, error){e.svc.Accept, e.svc.Decline} {
		b := e.apply(t, uuid.New())
		if _, err := decide(ctx, b.ID, e.author); err != nil {
			t.Fatal(err)
		}
		_, err := e.svc.Withdraw(ctx, b.ID, b.ApplicantID)
		wantErr(t, err, apperr.KindPrecondition, "Решение по отклику уже принято")
	}
}

func TestListByListing(t *testing.T) {
	ctx := context.Background()
	e := newEnv()

	// Zero applications: the author still gets an empty list, others get 403.
	got, err := e.svc.ListByListing(ctx, e.open, e.author, "")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
	_, err = e.svc.ListByListing(ctx, e.open, uuid.New(), "")
	wantErr(t, err, apperr.KindForbidden, "Недостаточно прав")
	_, err = e.svc.ListByListing(ctx, uuid.New(), e.author, "")
	wantErr(t, err, apperr.KindNotFound, "")

	a1 := e.apply(t, uuid.New())
	a2 := e.apply(t, uuid.New())
	a3 := e.apply(t, uuid.New())
	if _, err := e.svc.Accept(ctx, a3.ID, e.author); err != nil {
		t.Fatal(err)
	}
	// applicant of the listing is not its author either
	_, err = e.svc.ListByListing(ctx, e.open, a1.ApplicantID, "")
	wantErr(t, err, apperr.KindForbidden, "Недостаточно прав")

	got, err = e.svc.ListByListing(ctx, e.open, e.author, "")
	if err != nil || len(got) != 3 || got[0].ID != a2.ID || got[1].ID != a1.ID || got[2].ID != a3.ID {
		t.Fatalf("list = %v %v", got, err)
	}
	got, err = e.svc.ListByListing(ctx, e.open, e.author, applications.StatusAccepted)
	if err != nil || len(got) != 1 || got[0].ID != a3.ID {
		t.Fatalf("accepted = %v %v", got, err)
	}
	_, err = e.svc.ListByListing(ctx, e.open, e.author, "nope")
	wantErr(t, err, apperr.KindValidation, "")
}

func TestListsAndBatches(t *testing.T) {
	ctx := context.Background()
	e := newEnv()
	me := uuid.New()
	other := uuid.New()
	e.listings.items[other] = applications.ListingInfo{ID: other, AuthorID: e.author, IsOpen: true}

	mine := e.apply(t, me)
	theirs := e.apply(t, uuid.New())
	b, err := e.svc.Apply(ctx, other, me, "второй")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Withdraw(ctx, b.ID, me); err != nil {
		t.Fatal(err)
	}

	sent, err := e.svc.ListByApplicant(ctx, me, "")
	if err != nil || len(sent) != 2 || sent[0].ID != b.ID || sent[1].ID != mine.ID {
		t.Fatalf("sent = %v %v", sent, err)
	}
	sent, err = e.svc.ListByApplicant(ctx, me, applications.StatusPending)
	if err != nil || len(sent) != 1 || sent[0].ID != mine.ID || e.repo.listArg != applications.StatusPending {
		t.Fatalf("sent pending = %v %v", sent, err)
	}
	_, err = e.svc.ListByApplicant(ctx, me, "bogus")
	wantErr(t, err, apperr.KindValidation, "")

	received, err := e.svc.ListReceived(ctx, e.author, "")
	if err != nil || len(received) != 3 || received[2].ID != b.ID {
		t.Fatalf("received = %v %v", received, err)
	}
	if received[0].ID != theirs.ID || received[1].ID != mine.ID {
		t.Fatalf("received order = %v", received)
	}
	_, err = e.svc.ListReceived(ctx, e.author, "bogus")
	wantErr(t, err, apperr.KindValidation, "")

	m, err := e.svc.GetMineForListings(ctx, me, []uuid.UUID{e.open, other})
	if err != nil || len(m) != 1 || m[e.open] == nil || m[e.open].ID != mine.ID {
		t.Fatalf("mine = %v %v", m, err)
	}

	empty := uuid.New()
	st, err := e.svc.StatsForListings(ctx, []uuid.UUID{e.open, other, empty})
	if err != nil {
		t.Fatal(err)
	}
	if st[e.open] != (applications.Stats{Pending: 2, Total: 2}) || st[other] != (applications.Stats{}) {
		t.Fatalf("stats = %v", st)
	}
	if s, ok := st[empty]; !ok || s != (applications.Stats{}) {
		t.Fatalf("empty listing must have zero stats: %v", st)
	}
}
