package bff

import (
	"context"

	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// V1CreateApplication is not implemented yet (B4).
func (i *Implementation) V1CreateApplication(_ context.Context, _ *hbcv1.V1CreateApplicationRequest) (*hbcv1.Application, error) {
	return nil, errNotImplemented
}

// V1ListListingApplications is not implemented yet (B4).
func (i *Implementation) V1ListListingApplications(_ context.Context, _ *hbcv1.V1ListListingApplicationsRequest) (*hbcv1.V1ApplicationsResponse, error) {
	return nil, errNotImplemented
}

// V1AcceptApplication is not implemented yet (B4).
func (i *Implementation) V1AcceptApplication(_ context.Context, _ *hbcv1.V1ApplicationIDRequest) (*hbcv1.Application, error) {
	return nil, errNotImplemented
}

// V1DeclineApplication is not implemented yet (B4).
func (i *Implementation) V1DeclineApplication(_ context.Context, _ *hbcv1.V1ApplicationIDRequest) (*hbcv1.Application, error) {
	return nil, errNotImplemented
}

// V1WithdrawApplication is not implemented yet (B4).
func (i *Implementation) V1WithdrawApplication(_ context.Context, _ *hbcv1.V1ApplicationIDRequest) (*hbcv1.Application, error) {
	return nil, errNotImplemented
}

// V1ListSentApplications is not implemented yet (B4).
func (i *Implementation) V1ListSentApplications(_ context.Context, _ *hbcv1.V1ListSentApplicationsRequest) (*hbcv1.V1ApplicationsResponse, error) {
	return nil, errNotImplemented
}

// V1ListReceivedApplications is not implemented yet (B4).
func (i *Implementation) V1ListReceivedApplications(_ context.Context, _ *hbcv1.V1ListReceivedApplicationsRequest) (*hbcv1.V1ApplicationsResponse, error) {
	return nil, errNotImplemented
}
