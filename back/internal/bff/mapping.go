package bff

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// avatarPathPrefix is the API path that serves avatar files (V1GetAvatar).
const avatarPathPrefix = "/v1/avatars/"

// ---------- enums ----------

var (
	kindToProto = map[listings.Kind]hbcv1.ListingKind{
		listings.KindStartup: hbcv1.ListingKind_LISTING_KIND_STARTUP,
		listings.KindVacancy: hbcv1.ListingKind_LISTING_KIND_VACANCY,
	}
	kindFromProto = map[hbcv1.ListingKind]listings.Kind{
		hbcv1.ListingKind_LISTING_KIND_UNSPECIFIED: "",
		hbcv1.ListingKind_LISTING_KIND_STARTUP:     listings.KindStartup,
		hbcv1.ListingKind_LISTING_KIND_VACANCY:     listings.KindVacancy,
	}

	listingStatusToProto = map[listings.Status]hbcv1.ListingStatus{
		listings.StatusActive: hbcv1.ListingStatus_LISTING_STATUS_ACTIVE,
		listings.StatusClosed: hbcv1.ListingStatus_LISTING_STATUS_CLOSED,
	}
	listingStatusFromProto = map[hbcv1.ListingStatus]listings.Status{
		hbcv1.ListingStatus_LISTING_STATUS_UNSPECIFIED: "",
		hbcv1.ListingStatus_LISTING_STATUS_ACTIVE:      listings.StatusActive,
		hbcv1.ListingStatus_LISTING_STATUS_CLOSED:      listings.StatusClosed,
	}

	stageToProto = map[listings.Stage]hbcv1.StartupStage{
		listings.StageIdea:      hbcv1.StartupStage_STARTUP_STAGE_IDEA,
		listings.StagePrototype: hbcv1.StartupStage_STARTUP_STAGE_PROTOTYPE,
		listings.StageMVP:       hbcv1.StartupStage_STARTUP_STAGE_MVP,
		listings.StageLaunched:  hbcv1.StartupStage_STARTUP_STAGE_LAUNCHED,
	}
	stageFromProto = map[hbcv1.StartupStage]listings.Stage{
		hbcv1.StartupStage_STARTUP_STAGE_UNSPECIFIED: "",
		hbcv1.StartupStage_STARTUP_STAGE_IDEA:        listings.StageIdea,
		hbcv1.StartupStage_STARTUP_STAGE_PROTOTYPE:   listings.StagePrototype,
		hbcv1.StartupStage_STARTUP_STAGE_MVP:         listings.StageMVP,
		hbcv1.StartupStage_STARTUP_STAGE_LAUNCHED:    listings.StageLaunched,
	}

	appStatusToProto = map[applications.Status]hbcv1.ApplicationStatus{
		applications.StatusPending:   hbcv1.ApplicationStatus_APPLICATION_STATUS_PENDING,
		applications.StatusAccepted:  hbcv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED,
		applications.StatusDeclined:  hbcv1.ApplicationStatus_APPLICATION_STATUS_DECLINED,
		applications.StatusWithdrawn: hbcv1.ApplicationStatus_APPLICATION_STATUS_WITHDRAWN,
	}
	appStatusFromProto = map[hbcv1.ApplicationStatus]applications.Status{
		hbcv1.ApplicationStatus_APPLICATION_STATUS_UNSPECIFIED: "",
		hbcv1.ApplicationStatus_APPLICATION_STATUS_PENDING:     applications.StatusPending,
		hbcv1.ApplicationStatus_APPLICATION_STATUS_ACCEPTED:    applications.StatusAccepted,
		hbcv1.ApplicationStatus_APPLICATION_STATUS_DECLINED:    applications.StatusDeclined,
		hbcv1.ApplicationStatus_APPLICATION_STATUS_WITHDRAWN:   applications.StatusWithdrawn,
	}
)

// Domain → proto enum mappers. Unknown domain values map to UNSPECIFIED.

func kindProto(k listings.Kind) hbcv1.ListingKind                  { return kindToProto[k] }
func listingStatusProto(s listings.Status) hbcv1.ListingStatus     { return listingStatusToProto[s] }
func stageProto(s listings.Stage) hbcv1.StartupStage               { return stageToProto[s] }
func appStatusProto(s applications.Status) hbcv1.ApplicationStatus { return appStatusToProto[s] }

// ---------- scalars ----------

func tsProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func tsPtrProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return tsProto(*t)
}

// avatarURL builds User.avatar_url from the storage key ("" stays "").
func avatarURL(key string) string {
	if key == "" {
		return ""
	}
	return avatarPathPrefix + key
}

// ---------- entities ----------

func userProto(u *users.User) *hbcv1.User {
	if u == nil {
		return nil
	}
	return &hbcv1.User{
		Id:          u.ID.String(),
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Headline:    u.Headline,
		About:       u.About,
		Skills:      u.Skills,
		Telegram:    u.Telegram,
		CreatedAt:   tsProto(u.CreatedAt),
		AvatarUrl:   avatarURL(u.AvatarKey),
	}
}

func statsProto(s applications.Stats) *hbcv1.ListingStats {
	return &hbcv1.ListingStats{
		Pending:  int32(s.Pending),  //nolint:gosec // counters of one listing are far below int32
		Accepted: int32(s.Accepted), //nolint:gosec // see above
		Declined: int32(s.Declined), //nolint:gosec // see above
		Total:    int32(s.Total),    //nolint:gosec // see above
	}
}

// listingProto maps a listing with its composed parts. author, myApp and stats may be nil.
func listingProto(l *listings.Listing, me uuid.UUID, author *hbcv1.User, myApp *hbcv1.Application, stats *hbcv1.ListingStats) *hbcv1.Listing {
	return &hbcv1.Listing{
		Id:            l.ID.String(),
		Kind:          kindProto(l.Kind),
		Title:         l.Title,
		Description:   l.Description,
		LookingFor:    l.LookingFor,
		Tags:          l.Tags,
		Stage:         stageProto(l.Stage),
		Compensation:  l.Compensation,
		Status:        listingStatusProto(l.Status),
		Author:        author,
		CreatedAt:     tsProto(l.CreatedAt),
		UpdatedAt:     tsProto(l.UpdatedAt),
		ClosedAt:      tsPtrProto(l.ClosedAt),
		IsMine:        l.AuthorID == me,
		MyApplication: myApp,
		Stats:         stats,
	}
}

func listingSummaryProto(l *listings.Listing, author *hbcv1.User) *hbcv1.ListingSummary {
	if l == nil {
		return nil
	}
	return &hbcv1.ListingSummary{
		Id:     l.ID.String(),
		Kind:   kindProto(l.Kind),
		Title:  l.Title,
		Status: listingStatusProto(l.Status),
		Author: author,
	}
}

func applicationProto(a *applications.Application, listing *hbcv1.ListingSummary, applicant *hbcv1.User) *hbcv1.Application {
	return &hbcv1.Application{
		Id:        a.ID.String(),
		Listing:   listing,
		Applicant: applicant,
		Message:   a.Message,
		Status:    appStatusProto(a.Status),
		CreatedAt: tsProto(a.CreatedAt),
		UpdatedAt: tsProto(a.UpdatedAt),
		DecidedAt: tsPtrProto(a.DecidedAt),
	}
}
