package bff

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// Shape validation of proto requests. Content rules (lengths, duplicates, required
// stage, page/limit ranges) belong to the contexts and are not duplicated here.

// Feed paging defaults (SPEC §2.4): zero means "not set".
const (
	defaultPage  = 1
	defaultLimit = 20
)

// User-facing messages produced by the BFF itself.
const (
	msgKindRequired        = "Вид публикации: выберите стартап или вакансию"
	msgAvatarEmpty         = "Фото: файл пустой"
	msgBadRequest          = "Некорректный формат запроса"
	msgUserNotFound        = "Участник не найден"
	msgListingNotFound     = "Публикация не найдена"
	msgApplicationNotFound = "Отклик не найден"
	msgSessionExpired      = "Сессия истекла"
)

// maxRefreshTokenLen bounds the refresh token accepted from the client: issued tokens
// are 43 characters (base64url of 32 bytes), anything much longer cannot be ours.
const maxRefreshTokenLen = 128

// currentUser returns the authenticated caller. The auth interceptor has already
// rejected anonymous calls to private methods; this is a safety net.
func currentUser(ctx context.Context) (uuid.UUID, error) {
	id, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, auth.MsgUnauthenticated)
	}
	return id, nil
}

// refreshTokenParam checks the shape of a refresh token from the request: an empty or
// oversized one cannot name a session, so it is 401 «Сессия истекла».
func refreshTokenParam(token string) (string, error) {
	if token == "" || len(token) > maxRefreshTokenLen {
		return "", sessionExpired()
	}
	return token, nil
}

func sessionExpired() error { return status.Error(codes.Unauthenticated, msgSessionExpired) }

// parsePathID parses an id taken from the URL path. A malformed id cannot name any
// object, so it is reported as NotFound with the entity message (nothing to disclose).
func parsePathID(s, notFoundMsg string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, status.Error(codes.NotFound, notFoundMsg)
	}
	return id, nil
}

// badRequest is returned for enum values outside the contract (possible only for
// numeric values: names are checked by the gateway).
func badRequest() error { return status.Error(codes.InvalidArgument, msgBadRequest) }

// kindFilter maps the optional feed filter; UNSPECIFIED = all kinds.
func kindFilter(k hbcv1.ListingKind) (listings.Kind, error) {
	v, ok := kindFromProto[k]
	if !ok {
		return "", badRequest()
	}
	return v, nil
}

// requiredKind maps the kind of a new listing: UNSPECIFIED or unknown is rejected.
func requiredKind(k hbcv1.ListingKind) (listings.Kind, error) {
	v, ok := kindFromProto[k]
	if !ok || v == "" {
		return "", status.Error(codes.InvalidArgument, msgKindRequired)
	}
	return v, nil
}

// stageParam maps a startup stage; UNSPECIFIED = "" (the context decides whether it is required).
func stageParam(s hbcv1.StartupStage) (listings.Stage, error) {
	v, ok := stageFromProto[s]
	if !ok {
		return "", badRequest()
	}
	return v, nil
}

// listingStatusFilter maps an optional status filter; UNSPECIFIED = all.
func listingStatusFilter(s hbcv1.ListingStatus) (listings.Status, error) {
	v, ok := listingStatusFromProto[s]
	if !ok {
		return "", badRequest()
	}
	return v, nil
}

// appStatusFilter maps an optional status filter; UNSPECIFIED = all.
func appStatusFilter(s hbcv1.ApplicationStatus) (applications.Status, error) {
	v, ok := appStatusFromProto[s]
	if !ok {
		return "", badRequest()
	}
	return v, nil
}

// pageParams applies defaults to zero page/limit; other values go to the context as is.
func pageParams(page, limit int32) (p, l int) {
	p, l = int(page), int(limit)
	if p == 0 {
		p = defaultPage
	}
	if l == 0 {
		l = defaultLimit
	}
	return p, l
}
