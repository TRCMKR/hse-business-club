package bff

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/users"
)

// In-memory fakes of the context ports. They implement the rules of SPEC §2.3 that
// the BFF only passes through, with the same apperr kinds and messages the real
// contexts return, and count every port call.

// Messages of the fakes (the contexts own the real ones).
const (
	fmsgLoginTaken      = "Логин уже занят"
	fmsgBadLogin        = "Неверный логин или пароль"
	fmsgUsername        = "Логин: от 3 до 32 символов"
	fmsgDisplayName     = "Имя: от 1 до 80 символов"
	fmsgAvatarFormat    = "Фото: поддерживаются JPEG, PNG и WebP"
	fmsgAvatarNotFound  = "Фото не найдено"
	fmsgTitle           = "Название: от 3 до 120 символов"
	fmsgStageRequired   = "Стадия: укажите стадию"
	fmsgLimit           = "Размер страницы: от 1 до 50"
	fmsgForbiddenEdit   = "Редактировать публикацию может только автор"
	fmsgListingClosed   = "Публикация закрыта"
	fmsgOwnListing      = "Нельзя откликнуться на свою публикацию"
	fmsgAlreadyApplied  = "Вы уже откликались на эту публикацию"
	fmsgMessage         = "Сообщение: от 1 до 2000 символов"
	fmsgDecided         = "Решение по отклику уже принято"
	fmsgForbiddenListAp = "Смотреть отклики может только автор"
	fmsgNoRights        = "Недостаточно прав"
)

// notImage marks avatar input the fake users context rejects as a non-image.
var notImage = []byte("not an image")

// calls counts port calls by "Port.Method".
type calls struct {
	mu sync.Mutex
	m  map[string]int
}

func (c *calls) inc(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]int{}
	}
	c.m[name]++
}

func (c *calls) snapshot() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int, len(c.m))
	for k, v := range c.m {
		out[k] = v
	}
	return out
}

func (c *calls) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = nil
}

// clock returns strictly increasing times.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) tick() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.now.IsZero() {
		c.now = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	}
	c.now = c.now.Add(time.Minute)
	return c.now
}

// ---------- users ----------

type fakeUsers struct {
	calls *calls
	clock *clock

	mu        sync.Mutex
	byID      map[uuid.UUID]*users.User
	passwords map[uuid.UUID]string
	avatars   map[string][]byte
	refresh   map[string]*fakeRefresh // by token
	seq       int
	err       error // returned by every method when set
}

// fakeRefresh is a refresh token of the fake users context.
type fakeRefresh struct {
	userID  uuid.UUID
	family  int
	revoked bool
}

var _ users.Service = (*fakeUsers)(nil)

func newFakeUsers(c *calls, cl *clock) *fakeUsers {
	return &fakeUsers{
		calls: c, clock: cl, byID: map[uuid.UUID]*users.User{}, passwords: map[uuid.UUID]string{},
		avatars: map[string][]byte{}, refresh: map[string]*fakeRefresh{},
	}
}

func (f *fakeUsers) begin(name string) error {
	f.calls.inc("Users." + name)
	f.mu.Lock()
	return f.err
}

func cloneUser(u *users.User) *users.User {
	c := *u
	c.Skills = slices.Clone(u.Skills)
	return &c
}

func (f *fakeUsers) Register(_ context.Context, p users.RegisterParams) (*users.User, error) {
	err := f.begin("Register")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	name := strings.ToLower(p.Username)
	if len(name) < 3 || len(name) > 32 {
		return nil, apperr.Validation("username", fmsgUsername)
	}
	for _, u := range f.byID {
		if u.Username == name {
			return nil, apperr.New(apperr.KindConflict, fmsgLoginTaken)
		}
	}
	now := f.clock.tick()
	u := &users.User{ID: uuid.New(), Username: name, DisplayName: p.DisplayName, CreatedAt: now, UpdatedAt: now}
	f.byID[u.ID] = u
	f.passwords[u.ID] = p.Password
	return cloneUser(u), nil
}

func (f *fakeUsers) Authenticate(_ context.Context, username, password string) (*users.User, error) {
	err := f.begin("Authenticate")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	for _, u := range f.byID {
		if u.Username == strings.ToLower(username) && f.passwords[u.ID] == password {
			return cloneUser(u), nil
		}
	}
	return nil, apperr.New(apperr.KindUnauthenticated, fmsgBadLogin)
}

func (f *fakeUsers) Get(_ context.Context, id uuid.UUID) (*users.User, error) {
	err := f.begin("Get")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	u, ok := f.byID[id]
	if !ok {
		return nil, apperr.New(apperr.KindNotFound, msgUserNotFound)
	}
	return cloneUser(u), nil
}

func (f *fakeUsers) GetMany(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]*users.User, error) {
	err := f.begin("GetMany")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]*users.User{}
	for _, id := range ids {
		if u, ok := f.byID[id]; ok {
			out[id] = cloneUser(u)
		}
	}
	return out, nil
}

func (f *fakeUsers) UpdateProfile(_ context.Context, id uuid.UUID, p users.UpdateProfileParams) (*users.User, error) {
	err := f.begin("UpdateProfile")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	u, ok := f.byID[id]
	if !ok {
		return nil, apperr.New(apperr.KindNotFound, msgUserNotFound)
	}
	if p.DisplayName == "" {
		return nil, apperr.Validation("display_name", fmsgDisplayName)
	}
	u.DisplayName, u.Headline, u.About = p.DisplayName, p.Headline, p.About
	u.Skills, u.Telegram = slices.Clone(p.Skills), strings.TrimPrefix(p.Telegram, "@")
	u.UpdatedAt = f.clock.tick()
	return cloneUser(u), nil
}

func (f *fakeUsers) SetAvatar(_ context.Context, id uuid.UUID, image []byte) (*users.User, error) {
	err := f.begin("SetAvatar")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	u, ok := f.byID[id]
	if !ok {
		return nil, apperr.New(apperr.KindNotFound, msgUserNotFound)
	}
	if string(image) == string(notImage) {
		return nil, apperr.Validation("avatar", fmsgAvatarFormat)
	}
	delete(f.avatars, u.AvatarKey)
	u.AvatarKey = uuid.NewString() + ".jpg"
	f.avatars[u.AvatarKey] = slices.Clone(image)
	return cloneUser(u), nil
}

func (f *fakeUsers) RemoveAvatar(_ context.Context, id uuid.UUID) (*users.User, error) {
	err := f.begin("RemoveAvatar")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	u, ok := f.byID[id]
	if !ok {
		return nil, apperr.New(apperr.KindNotFound, msgUserNotFound)
	}
	delete(f.avatars, u.AvatarKey)
	u.AvatarKey = ""
	return cloneUser(u), nil
}

func (f *fakeUsers) OpenAvatar(_ context.Context, key string) (*users.Avatar, error) {
	err := f.begin("OpenAvatar")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	data, ok := f.avatars[key]
	if !ok {
		return nil, apperr.New(apperr.KindNotFound, fmsgAvatarNotFound)
	}
	return &users.Avatar{ContentType: jpegType, Data: slices.Clone(data)}, nil
}

// newRefresh issues "refresh:<n>" in family (0 = a new family). f.mu is held.
func (f *fakeUsers) newRefresh(userID uuid.UUID, family int) *users.RefreshToken {
	f.seq++
	if family == 0 {
		family = f.seq
	}
	token := fmt.Sprintf("refresh:%d", f.seq)
	f.refresh[token] = &fakeRefresh{userID: userID, family: family}
	return &users.RefreshToken{Token: token, ExpiresAt: f.clock.tick().Add(time.Hour)}
}

func (f *fakeUsers) revokeFamily(family int) {
	for _, r := range f.refresh {
		if r.family == family {
			r.revoked = true
		}
	}
}

func (f *fakeUsers) IssueRefresh(_ context.Context, userID uuid.UUID) (*users.RefreshToken, error) {
	err := f.begin("IssueRefresh")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.newRefresh(userID, 0), nil
}

func (f *fakeUsers) RotateRefresh(_ context.Context, token string) (uuid.UUID, *users.RefreshToken, error) {
	err := f.begin("RotateRefresh")
	defer f.mu.Unlock()
	if err != nil {
		return uuid.Nil, nil, err
	}
	cur, ok := f.refresh[token]
	if !ok {
		return uuid.Nil, nil, apperr.New(apperr.KindUnauthenticated, msgSessionExpired)
	}
	if cur.revoked {
		f.revokeFamily(cur.family)
		return uuid.Nil, nil, apperr.New(apperr.KindUnauthenticated, msgSessionExpired)
	}
	cur.revoked = true
	return cur.userID, f.newRefresh(cur.userID, cur.family), nil
}

func (f *fakeUsers) RevokeRefresh(_ context.Context, token string) error {
	err := f.begin("RevokeRefresh")
	defer f.mu.Unlock()
	if err != nil {
		return err
	}
	if cur, ok := f.refresh[token]; ok {
		f.revokeFamily(cur.family)
	}
	return nil
}

// ---------- listings ----------

type fakeListings struct {
	calls *calls
	clock *clock

	mu    sync.Mutex
	byID  map[uuid.UUID]*listings.Listing
	order []uuid.UUID // creation order
	err   error
}

var _ listings.Service = (*fakeListings)(nil)

func newFakeListings(c *calls, cl *clock) *fakeListings {
	return &fakeListings{calls: c, clock: cl, byID: map[uuid.UUID]*listings.Listing{}}
}

func (f *fakeListings) begin(name string) error {
	f.calls.inc("Listings." + name)
	f.mu.Lock()
	return f.err
}

func cloneListing(l *listings.Listing) *listings.Listing {
	c := *l
	c.Tags = slices.Clone(l.Tags)
	if l.ClosedAt != nil {
		t := *l.ClosedAt
		c.ClosedAt = &t
	}
	return &c
}

func validateListing(kind listings.Kind, title string, stage listings.Stage) error {
	if n := len([]rune(title)); n < 3 || n > 120 {
		return apperr.Validation("title", fmsgTitle)
	}
	if kind == listings.KindStartup && stage == "" {
		return apperr.Validation("stage", fmsgStageRequired)
	}
	return nil
}

func (f *fakeListings) Create(_ context.Context, p listings.CreateParams) (*listings.Listing, error) {
	err := f.begin("Create")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := validateListing(p.Kind, p.Title, p.Stage); err != nil {
		return nil, err
	}
	now := f.clock.tick()
	l := &listings.Listing{
		ID: uuid.New(), AuthorID: p.AuthorID, Kind: p.Kind, Title: p.Title, Description: p.Description,
		LookingFor: p.LookingFor, Tags: slices.Clone(p.Tags), Stage: p.Stage, Compensation: p.Compensation,
		Status: listings.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	f.byID[l.ID] = l
	f.order = append(f.order, l.ID)
	return cloneListing(l), nil
}

func (f *fakeListings) Get(_ context.Context, id uuid.UUID) (*listings.Listing, error) {
	err := f.begin("Get")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	l, ok := f.byID[id]
	if !ok {
		return nil, apperr.New(apperr.KindNotFound, msgListingNotFound)
	}
	return cloneListing(l), nil
}

func (f *fakeListings) GetMany(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]*listings.Listing, error) {
	err := f.begin("GetMany")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]*listings.Listing{}
	for _, id := range ids {
		if l, ok := f.byID[id]; ok {
			out[id] = cloneListing(l)
		}
	}
	return out, nil
}

// newestFirst returns listings matching keep in created_at DESC order. Callers hold mu.
func (f *fakeListings) newestFirst(keep func(*listings.Listing) bool) []*listings.Listing {
	var out []*listings.Listing
	for i := len(f.order) - 1; i >= 0; i-- {
		if l := f.byID[f.order[i]]; keep(l) {
			out = append(out, cloneListing(l))
		}
	}
	return out
}

func (f *fakeListings) ListActive(_ context.Context, flt listings.ListFilter) (*listings.Page, error) {
	err := f.begin("ListActive")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if flt.Limit < 1 || flt.Limit > 50 {
		return nil, apperr.Validation("limit", fmsgLimit)
	}
	q := strings.ToLower(flt.Query)
	all := f.newestFirst(func(l *listings.Listing) bool {
		return l.Status == listings.StatusActive &&
			(flt.Kind == "" || l.Kind == flt.Kind) &&
			(q == "" || strings.Contains(strings.ToLower(l.Title+" "+l.Description), q))
	})
	from := min((flt.Page-1)*flt.Limit, len(all))
	to := min(from+flt.Limit, len(all))
	return &listings.Page{Items: all[from:to], Total: len(all), Page: flt.Page, Limit: flt.Limit}, nil
}

func (f *fakeListings) ListByAuthor(_ context.Context, authorID uuid.UUID, status listings.Status) ([]*listings.Listing, error) {
	err := f.begin("ListByAuthor")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.newestFirst(func(l *listings.Listing) bool {
		return l.AuthorID == authorID && (status == "" || l.Status == status)
	}), nil
}

func (f *fakeListings) Update(_ context.Context, id, actorID uuid.UUID, p listings.UpdateParams) (*listings.Listing, error) {
	err := f.begin("Update")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	l, ok := f.byID[id]
	switch {
	case !ok:
		return nil, apperr.New(apperr.KindNotFound, msgListingNotFound)
	case l.AuthorID != actorID:
		return nil, apperr.New(apperr.KindForbidden, fmsgForbiddenEdit)
	case l.Status != listings.StatusActive:
		return nil, apperr.New(apperr.KindPrecondition, fmsgListingClosed)
	}
	if err := validateListing(l.Kind, p.Title, p.Stage); err != nil {
		return nil, err
	}
	l.Title, l.Description, l.LookingFor = p.Title, p.Description, p.LookingFor
	l.Tags, l.Stage, l.Compensation = slices.Clone(p.Tags), p.Stage, p.Compensation
	l.UpdatedAt = f.clock.tick()
	return cloneListing(l), nil
}

func (f *fakeListings) Close(_ context.Context, id, actorID uuid.UUID) (*listings.Listing, error) {
	err := f.begin("Close")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	l, ok := f.byID[id]
	switch {
	case !ok:
		return nil, apperr.New(apperr.KindNotFound, msgListingNotFound)
	case l.AuthorID != actorID:
		return nil, apperr.New(apperr.KindForbidden, fmsgForbiddenEdit)
	}
	if l.Status == listings.StatusActive {
		now := f.clock.tick()
		l.Status, l.ClosedAt, l.UpdatedAt = listings.StatusClosed, &now, now
	}
	return cloneListing(l), nil
}

// info is the applications.ListingReader view (no call counting: it is an adapter).
func (f *fakeListings) info(id uuid.UUID) (applications.ListingInfo, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, ok := f.byID[id]
	if !ok {
		return applications.ListingInfo{}, false
	}
	return applications.ListingInfo{ID: l.ID, AuthorID: l.AuthorID, IsOpen: l.Status == listings.StatusActive}, true
}

// ---------- applications ----------

type fakeApplications struct {
	calls    *calls
	clock    *clock
	listings *fakeListings

	mu    sync.Mutex
	byID  map[uuid.UUID]*applications.Application
	order []uuid.UUID
	err   error
}

var _ applications.Service = (*fakeApplications)(nil)

func newFakeApplications(c *calls, cl *clock, ls *fakeListings) *fakeApplications {
	return &fakeApplications{calls: c, clock: cl, listings: ls, byID: map[uuid.UUID]*applications.Application{}}
}

func (f *fakeApplications) begin(name string) error {
	f.calls.inc("Applications." + name)
	f.mu.Lock()
	return f.err
}

func cloneApp(a *applications.Application) *applications.Application {
	c := *a
	if a.DecidedAt != nil {
		t := *a.DecidedAt
		c.DecidedAt = &t
	}
	return &c
}

func (f *fakeApplications) Apply(_ context.Context, listingID, applicantID uuid.UUID, message string) (*applications.Application, error) {
	err := f.begin("Apply")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	info, ok := f.listings.info(listingID)
	switch {
	case !ok:
		return nil, apperr.New(apperr.KindNotFound, msgListingNotFound)
	case info.AuthorID == applicantID:
		return nil, apperr.New(apperr.KindPrecondition, fmsgOwnListing)
	case !info.IsOpen:
		return nil, apperr.New(apperr.KindPrecondition, fmsgListingClosed)
	case message == "":
		return nil, apperr.Validation("message", fmsgMessage)
	}
	for _, a := range f.byID {
		if a.ListingID == listingID && a.ApplicantID == applicantID && a.Status != applications.StatusWithdrawn {
			return nil, apperr.New(apperr.KindConflict, fmsgAlreadyApplied)
		}
	}
	now := f.clock.tick()
	a := &applications.Application{
		ID: uuid.New(), ListingID: listingID, ListingAuthorID: info.AuthorID, ApplicantID: applicantID,
		Message: message, Status: applications.StatusPending, CreatedAt: now, UpdatedAt: now,
	}
	f.byID[a.ID] = a
	f.order = append(f.order, a.ID)
	return cloneApp(a), nil
}

func (f *fakeApplications) Get(_ context.Context, id, actorID uuid.UUID) (*applications.Application, error) {
	err := f.begin("Get")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	a, ok := f.byID[id]
	if !ok || (a.ApplicantID != actorID && a.ListingAuthorID != actorID) {
		return nil, apperr.New(apperr.KindNotFound, msgApplicationNotFound)
	}
	return cloneApp(a), nil
}

func (f *fakeApplications) decide(name string, id, actorID uuid.UUID, to applications.Status) (*applications.Application, error) {
	err := f.begin(name)
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	a, ok := f.byID[id]
	switch {
	case !ok || (a.ListingAuthorID != actorID && a.ApplicantID != actorID):
		return nil, apperr.New(apperr.KindNotFound, msgApplicationNotFound)
	case a.ListingAuthorID != actorID: // the applicant cannot decide
		return nil, apperr.New(apperr.KindForbidden, fmsgNoRights)
	}
	switch a.Status {
	case to:
	case applications.StatusPending:
		now := f.clock.tick()
		a.Status, a.DecidedAt, a.UpdatedAt = to, &now, now
	default:
		return nil, apperr.New(apperr.KindPrecondition, fmsgDecided)
	}
	return cloneApp(a), nil
}

func (f *fakeApplications) Accept(_ context.Context, id, actorID uuid.UUID) (*applications.Application, error) {
	return f.decide("Accept", id, actorID, applications.StatusAccepted)
}

func (f *fakeApplications) Decline(_ context.Context, id, actorID uuid.UUID) (*applications.Application, error) {
	return f.decide("Decline", id, actorID, applications.StatusDeclined)
}

func (f *fakeApplications) Withdraw(_ context.Context, id, actorID uuid.UUID) (*applications.Application, error) {
	err := f.begin("Withdraw")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	a, ok := f.byID[id]
	switch {
	case !ok || (a.ListingAuthorID != actorID && a.ApplicantID != actorID):
		return nil, apperr.New(apperr.KindNotFound, msgApplicationNotFound)
	case a.ApplicantID != actorID: // the listing author cannot withdraw
		return nil, apperr.New(apperr.KindForbidden, fmsgNoRights)
	}
	switch a.Status {
	case applications.StatusWithdrawn:
	case applications.StatusPending:
		a.Status, a.UpdatedAt = applications.StatusWithdrawn, f.clock.tick()
	default:
		return nil, apperr.New(apperr.KindPrecondition, fmsgDecided)
	}
	return cloneApp(a), nil
}

// list returns applications matching keep; pendingFirst orders pending ones first,
// then created_at DESC. Callers hold mu.
func (f *fakeApplications) list(keep func(*applications.Application) bool, pendingFirst bool) []*applications.Application {
	var out []*applications.Application
	for i := len(f.order) - 1; i >= 0; i-- {
		if a := f.byID[f.order[i]]; keep(a) {
			out = append(out, cloneApp(a))
		}
	}
	if pendingFirst {
		slices.SortStableFunc(out, func(a, b *applications.Application) int {
			ap, bp := a.Status == applications.StatusPending, b.Status == applications.StatusPending
			switch {
			case ap && !bp:
				return -1
			case !ap && bp:
				return 1
			}
			return 0
		})
	}
	return out
}

func statusMatches(want, got applications.Status) bool { return want == "" || want == got }

func (f *fakeApplications) ListByListing(_ context.Context, listingID, actorID uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	err := f.begin("ListByListing")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	info, ok := f.listings.info(listingID)
	switch {
	case !ok:
		return nil, apperr.New(apperr.KindNotFound, msgListingNotFound)
	case info.AuthorID != actorID:
		return nil, apperr.New(apperr.KindForbidden, fmsgForbiddenListAp)
	}
	return f.list(func(a *applications.Application) bool {
		return a.ListingID == listingID && statusMatches(status, a.Status)
	}, true), nil
}

func (f *fakeApplications) ListByApplicant(_ context.Context, applicantID uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	err := f.begin("ListByApplicant")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.list(func(a *applications.Application) bool {
		return a.ApplicantID == applicantID && statusMatches(status, a.Status)
	}, false), nil
}

func (f *fakeApplications) ListReceived(_ context.Context, authorID uuid.UUID, status applications.Status) ([]*applications.Application, error) {
	err := f.begin("ListReceived")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.list(func(a *applications.Application) bool {
		return a.ListingAuthorID == authorID && statusMatches(status, a.Status)
	}, true), nil
}

func (f *fakeApplications) GetMineForListings(_ context.Context, applicantID uuid.UUID, listingIDs []uuid.UUID) (map[uuid.UUID]*applications.Application, error) {
	err := f.begin("GetMineForListings")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]*applications.Application{}
	for _, a := range f.byID {
		if a.ApplicantID == applicantID && a.Status != applications.StatusWithdrawn && slices.Contains(listingIDs, a.ListingID) {
			out[a.ListingID] = cloneApp(a)
		}
	}
	return out, nil
}

func (f *fakeApplications) StatsForListings(_ context.Context, listingIDs []uuid.UUID) (map[uuid.UUID]applications.Stats, error) {
	err := f.begin("StatsForListings")
	defer f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]applications.Stats, len(listingIDs))
	for _, id := range listingIDs {
		out[id] = applications.Stats{}
	}
	for _, a := range f.byID {
		s, ok := out[a.ListingID]
		if !ok {
			continue
		}
		switch a.Status {
		case applications.StatusPending:
			s.Pending++
		case applications.StatusAccepted:
			s.Accepted++
		case applications.StatusDeclined:
			s.Declined++
		case applications.StatusWithdrawn:
			continue
		}
		s.Total = s.Pending + s.Accepted + s.Declined
		out[a.ListingID] = s
	}
	return out, nil
}

// ---------- auth ----------

// fakeIssuer issues "token:<user id>" tokens and verifies them back.
type fakeIssuer struct{ err error }

func (f fakeIssuer) Issue(id uuid.UUID) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "token:" + id.String(), nil
}

func (fakeIssuer) Verify(token string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimPrefix(token, "token:"))
	if err != nil || !strings.HasPrefix(token, "token:") {
		return uuid.Nil, errors.New("fake: invalid token")
	}
	return id, nil
}
