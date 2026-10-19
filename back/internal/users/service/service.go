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
	repo   Repository
	hasher Hasher
	store  blob.Store

	dummyHash string // hash of a random password, compared on misses
}

var _ users.Service = (*Service)(nil)

// fallbackDummyHash is a valid bcrypt cost-10 hash of a password nobody uses; it is
// the miss hash when hasher.Hash fails in New, so a miss still costs a full comparison.
const fallbackDummyHash = "$2a$10$THdrbhnK4GNQsTumdSCR3.kJ66ed.TkcKVhO50YoT/Q9ZVAt.XVYe" //nolint:gosec // not a credential

// New returns the users service. The hash compared on login misses is computed here,
// once, so that the first miss costs the same as any other.
func New(repo Repository, hasher Hasher, store blob.Store) users.Service {
	dummy, err := hasher.Hash(uuid.NewString())
	if err != nil || dummy == "" {
		logger.L().Error("users: dummy password hash, using the built-in one", zap.Error(err))
		dummy = fallbackDummyHash
	}
	return &Service{repo: repo, hasher: hasher, store: store, dummyHash: dummy}
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

func (s *Service) deleteObject(ctx context.Context, key string) {
	if err := s.store.Delete(ctx, key); err != nil {
		logger.FromContext(ctx).Warn("users: delete avatar object", zap.String("key", key), zap.Error(err))
	}
}
