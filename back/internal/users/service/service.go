// Package service implements users.Service on top of the repository, a password
// hasher and the blob store for profile photos.
package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/trcmkr/hse-business-club/back/internal/platform/apperr"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/platform/blob"
	"github.com/trcmkr/hse-business-club/back/internal/platform/logger"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	"github.com/trcmkr/hse-business-club/back/internal/users/domain"
)

// User-facing messages of the service.
const (
	MsgInvalidCredentials = "Неверный логин или пароль" //nolint:gosec // user-facing message, not a credential
	MsgAvatarNotFound     = "Фото не найдено"
	MsgSessionExpired     = "Сессия истекла"
	msgInternal           = "Внутренняя ошибка"
)

// maxStoredAvatar bounds how much of a stored photo is read into memory. A normalized
// 512×512 JPEG q85 is ~20–150 KB; anything above this is not ours.
const maxStoredAvatar = 1 << 20

var avatarKeyRe = regexp.MustCompile(`^[0-9a-f-]{36}\.jpg$`)

// Repository is the storage the service needs; postgres.Repository implements it.
// Errors are *apperr.Error (KindNotFound, KindConflict, KindUnknown).
type Repository interface {
	Create(ctx context.Context, username, passwordHash, displayName string) (*users.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*users.User, error)
	// GetByUsername also returns the password hash; it never leaves this package.
	GetByUsername(ctx context.Context, username string) (*users.User, string, error)
	GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*users.User, error)
	Update(ctx context.Context, id uuid.UUID, p users.UpdateProfileParams) (*users.User, error)
	SetAvatarKey(ctx context.Context, id uuid.UUID, key string) (*users.User, error)
}

// RefreshRepository stores refresh tokens; postgres.RefreshRepository implements it.
// Errors are *apperr.Error (KindNotFound, KindConflict, KindUnknown).
type RefreshRepository interface {
	// InTx runs fn in one transaction; the methods called with fn's ctx use it.
	// Errors of fn are returned as is.
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
	Create(ctx context.Context, t *domain.StoredRefreshToken) (*domain.StoredRefreshToken, error)
	// GetByHash returns active, revoked and expired tokens alike and locks the row
	// for the rest of the transaction. KindNotFound if there is none.
	GetByHash(ctx context.Context, hash []byte) (*domain.StoredRefreshToken, error)
	RevokeByID(ctx context.Context, id uuid.UUID) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID) error
	DeleteExpired(ctx context.Context, userID uuid.UUID, now time.Time) error
}

// DefaultRefreshTTL is the refresh token lifetime unless WithRefreshTTL is given
// (REFRESH_TTL default, ARCHITECTURE §5).
const DefaultRefreshTTL = 720 * time.Hour

// Option configures the Service.
type Option func(*Service)

// WithRefreshTTL sets the refresh token lifetime (REFRESH_TTL); non-positive values
// are ignored.
func WithRefreshTTL(d time.Duration) Option {
	return func(s *Service) {
		if d > 0 {
			s.refreshTTL = d
		}
	}
}

// WithClock sets the current time source (for tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// Hasher hashes and checks passwords.
type Hasher interface {
	Hash(password string) (string, error)
	Check(hash, password string) bool
}

// Bcrypt is the production Hasher (bcrypt cost 10, platform/auth).
var Bcrypt Hasher = bcryptHasher{}

type bcryptHasher struct{}

func (bcryptHasher) Hash(password string) (string, error) { return auth.HashPassword(password) }
func (bcryptHasher) Check(hash, password string) bool     { return auth.CheckPassword(hash, password) }

// Service implements users.Service.
type Service struct {
	repo       Repository
	tokens     RefreshRepository
	hasher     Hasher
	store      blob.Store
	refreshTTL time.Duration
	now        func() time.Time

	dummyHash string // hash of a random password, compared on misses
}

var _ users.Service = (*Service)(nil)

// fallbackDummyHash is a valid bcrypt cost-10 hash of a password nobody uses; it is
// the miss hash when hasher.Hash fails in New, so a miss still costs a full comparison.
const fallbackDummyHash = "$2a$10$THdrbhnK4GNQsTumdSCR3.kJ66ed.TkcKVhO50YoT/Q9ZVAt.XVYe" //nolint:gosec // not a credential

// New returns the users service. The hash compared on login misses is computed here,
// once, so that the first miss costs the same as any other.
func New(repo Repository, tokens RefreshRepository, hasher Hasher, store blob.Store, opts ...Option) users.Service {
	dummy, err := hasher.Hash(uuid.NewString())
	if err != nil || dummy == "" {
		logger.L().Error("users: dummy password hash, using the built-in one", zap.Error(err))
		dummy = fallbackDummyHash
	}
	s := &Service{
		repo: repo, tokens: tokens, hasher: hasher, store: store,
		refreshTTL: DefaultRefreshTTL, now: time.Now, dummyHash: dummy,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Register implements users.Service.
func (s *Service) Register(ctx context.Context, p users.RegisterParams) (*users.User, error) {
	p, err := domain.NormalizeRegister(p)
	if err != nil {
		return nil, err
	}
	hash, err := s.hasher.Hash(p.Password)
	if err != nil {
		return nil, apperr.Wrap(apperr.KindUnknown, msgInternal, fmt.Errorf("hash password: %w", err))
	}
	return s.repo.Create(ctx, p.Username, hash, p.DisplayName)
}

// Authenticate implements users.Service. A malformed login, an unknown login and a wrong
// password give the same error, and a password hash is compared in every case so the
// response time does not reveal whether the login exists. Storage failures are not
// masked: they are KindUnknown (no information about the login either).
func (s *Service) Authenticate(ctx context.Context, username, password string) (*users.User, error) {
	username = domain.NormalizeUsername(username)
	if domain.ValidateUsername(username) != nil {
		s.hasher.Check(s.dummyHash, password)
		return nil, apperr.New(apperr.KindUnauthenticated, MsgInvalidCredentials)
	}
	u, hash, err := s.repo.GetByUsername(ctx, username)
	switch {
	case apperr.KindOf(err) == apperr.KindNotFound:
		s.hasher.Check(s.dummyHash, password)
		return nil, apperr.New(apperr.KindUnauthenticated, MsgInvalidCredentials)
	case err != nil:
		return nil, err
	}
	if !s.hasher.Check(hash, password) {
		return nil, apperr.New(apperr.KindUnauthenticated, MsgInvalidCredentials)
	}
	return u, nil
}

// Get implements users.Service.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*users.User, error) {
	return s.repo.GetByID(ctx, id)
}

// GetMany implements users.Service.
func (s *Service) GetMany(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*users.User, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]*users.User{}, nil
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	uniq := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			uniq = append(uniq, id)
		}
	}
	return s.repo.GetMany(ctx, uniq)
}

// UpdateProfile implements users.Service.
func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, p users.UpdateProfileParams) (*users.User, error) {
	p, err := domain.NormalizeProfile(p)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, p)
}

// SetAvatar implements users.Service: normalize → put under a new key → save the key →
// delete the previous object (a failed delete is only logged).
func (s *Service) SetAvatar(ctx context.Context, id uuid.UUID, image []byte) (*users.User, error) {
	data, err := domain.NormalizeAvatar(image)
	if err != nil {
		return nil, err
	}
	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	key := uuid.NewString() + ".jpg"
	if err := s.store.Put(ctx, key, domain.AvatarMediaType, bytes.NewReader(data), int64(len(data))); err != nil {
		return nil, apperr.Wrap(apperr.KindUnknown, msgInternal, fmt.Errorf("put avatar: %w", err))
	}
	u, err := s.repo.SetAvatarKey(ctx, id, key)
	if err != nil {
		s.deleteObject(ctx, key)
		return nil, err
	}
	if cur.AvatarKey != "" && cur.AvatarKey != key {
		s.deleteObject(ctx, cur.AvatarKey)
	}
	return u, nil
}

// RemoveAvatar implements users.Service. Without a photo the profile is returned as is.
func (s *Service) RemoveAvatar(ctx context.Context, id uuid.UUID) (*users.User, error) {
	cur, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cur.AvatarKey == "" {
		return cur, nil
	}
	u, err := s.repo.SetAvatarKey(ctx, id, "")
	if err != nil {
		return nil, err
	}
	s.deleteObject(ctx, cur.AvatarKey)
	return u, nil
}

// OpenAvatar implements users.Service.
func (s *Service) OpenAvatar(ctx context.Context, key string) (*users.Avatar, error) {
	if !avatarKeyRe.MatchString(key) {
		return nil, apperr.New(apperr.KindNotFound, MsgAvatarNotFound)
	}
	obj, err := s.store.Get(ctx, key)
	if errors.Is(err, blob.ErrNotFound) {
		return nil, apperr.Wrap(apperr.KindNotFound, MsgAvatarNotFound, err)
	}
	if err != nil {
		return nil, apperr.Wrap(apperr.KindUnknown, msgInternal, fmt.Errorf("get avatar: %w", err))
	}
	defer obj.Body.Close()
	data, err := io.ReadAll(io.LimitReader(obj.Body, maxStoredAvatar+1))
	if err != nil {
		return nil, apperr.Wrap(apperr.KindUnknown, msgInternal, fmt.Errorf("read avatar: %w", err))
	}
	if len(data) > maxStoredAvatar {
		return nil, apperr.Wrap(apperr.KindUnknown, msgInternal, fmt.Errorf("avatar %s is larger than %d bytes", key, maxStoredAvatar))
	}
	return &users.Avatar{ContentType: domain.AvatarMediaType, Data: data}, nil
}

// IssueRefresh implements users.Service: a new token in a new family.
func (s *Service) IssueRefresh(ctx context.Context, userID uuid.UUID) (*users.RefreshToken, error) {
	return s.issueRefresh(ctx, userID, uuid.New())
}

// RotateRefresh implements users.Service. In one transaction: find the token by hash
// (locking it, so concurrent rotations of one token are serialized), reject unknown and
// expired ones, revoke the whole family on reuse of a revoked token (the revocation is
// committed, the caller gets 401), otherwise revoke the token, drop the user's expired
// tokens and issue the next token of the same family.
func (s *Service) RotateRefresh(ctx context.Context, token string) (uuid.UUID, *users.RefreshToken, error) {
	if token == "" {
		return uuid.Nil, nil, sessionExpired()
	}
	hash := domain.HashRefreshToken(token)
	var (
		userID uuid.UUID
		next   *users.RefreshToken
		reused *domain.StoredRefreshToken
	)
	err := s.tokens.InTx(ctx, func(ctx context.Context) error {
		cur, err := s.tokens.GetByHash(ctx, hash)
		if apperr.KindOf(err) == apperr.KindNotFound {
			return sessionExpired()
		}
		if err != nil {
			return err
		}
		if cur.RevokedAt != nil {
			reused = cur
			return s.tokens.RevokeFamily(ctx, cur.FamilyID)
		}
		now := s.now()
		if !now.Before(cur.ExpiresAt) {
			return sessionExpired()
		}
		if err := s.tokens.RevokeByID(ctx, cur.ID); err != nil {
			return err
		}
		if err := s.tokens.DeleteExpired(ctx, cur.UserID, now); err != nil {
			return err
		}
		next, err = s.issueRefresh(ctx, cur.UserID, cur.FamilyID)
		userID = cur.UserID
		return err
	})
	if err != nil {
		return uuid.Nil, nil, err
	}
	if reused != nil {
		logger.FromContext(ctx).Warn("users: revoked refresh token reused, family revoked",
			zap.Stringer("user_id", reused.UserID), zap.Stringer("family_id", reused.FamilyID))
		return uuid.Nil, nil, sessionExpired()
	}
	return userID, next, nil
}

// RevokeRefresh implements users.Service: revokes the token's family (the whole
// sign-in). Empty and unknown tokens are not an error.
func (s *Service) RevokeRefresh(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	cur, err := s.tokens.GetByHash(ctx, domain.HashRefreshToken(token))
	if apperr.KindOf(err) == apperr.KindNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	return s.tokens.RevokeFamily(ctx, cur.FamilyID)
}

func (s *Service) issueRefresh(ctx context.Context, userID, familyID uuid.UUID) (*users.RefreshToken, error) {
	token, hash, err := domain.NewRefreshToken()
	if err != nil {
		return nil, apperr.Wrap(apperr.KindUnknown, msgInternal, err)
	}
	stored, err := s.tokens.Create(ctx, &domain.StoredRefreshToken{
		UserID: userID, FamilyID: familyID, Hash: hash, ExpiresAt: s.now().Add(s.refreshTTL),
	})
	if err != nil {
		return nil, err
	}
	return &users.RefreshToken{Token: token, ExpiresAt: stored.ExpiresAt}, nil
}

func sessionExpired() error { return apperr.New(apperr.KindUnauthenticated, MsgSessionExpired) }

func (s *Service) deleteObject(ctx context.Context, key string) {
	if err := s.store.Delete(ctx, key); err != nil {
		logger.FromContext(ctx).Warn("users: delete avatar object", zap.String("key", key), zap.Error(err))
	}
}
