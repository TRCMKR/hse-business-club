//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/listings/postgres"
	"github.com/trcmkr/hse-business-club/back/internal/listings/service"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/db/dbtest"
)

// TestServiceOnPostgres runs the main service flow on the real repository. It lives in
// this package because all listings integration tests share the "listings" schema and
// must not run in parallel packages.
func TestServiceOnPostgres(t *testing.T) {
	pool := dbtest.Pool(t)
	dbtest.Migrate(t, pool, postgres.Migrator)
	svc := service.New(postgres.New(pool))
	ctx := context.Background()
	author := uuid.New()
	upd := updParams()

	l, err := svc.Create(ctx, params(author, listings.KindStartup, "Стартап", "Описание стартапа"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, params(author, listings.KindStartup, "Стартап", "Описание стартапа")); err != nil {
		t.Fatal(err)
	}

	page, err := svc.ListActive(ctx, listings.ListFilter{Page: 1000, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if page.Items == nil || len(page.Items) != 0 || page.Total != 2 || page.Page != 1000 || page.Limit != 50 {
		t.Fatalf("page 1000 = %+v", page)
	}

	if _, err := svc.Update(ctx, l.ID, uuid.New(), upd); apperr.KindOf(err) != apperr.KindForbidden {
		t.Fatalf("Update(non-author) = %v", err)
	}
	if _, err := svc.Update(ctx, l.ID, author, upd); err != nil {
		t.Fatal(err)
	}
	c1, err := svc.Close(ctx, l.ID, author)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := svc.Close(ctx, l.ID, author)
	if err != nil || !c2.ClosedAt.Equal(*c1.ClosedAt) {
		t.Fatalf("second Close = %+v, %v", c2, err)
	}
	if _, err := svc.Update(ctx, l.ID, author, upd); apperr.KindOf(err) != apperr.KindPrecondition {
		t.Fatalf("Update(closed) = %v", err)
	}
	page, err = svc.ListActive(ctx, listings.ListFilter{})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID == l.ID {
		t.Fatalf("feed after close = %+v, %v", page, err)
	}
}
