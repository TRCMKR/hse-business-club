// Package bff implements the public gRPC API hbc.v1 (served over HTTP/JSON by
// grpc-gateway): request validation, caller authorization, response composition
// across contexts and error mapping. It talks to contexts only through their ports.
package bff

import (
	"context"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"github.com/trcmkr/hse-business-club/back/internal/applications"
	"github.com/trcmkr/hse-business-club/back/internal/listings"
	"github.com/trcmkr/hse-business-club/back/internal/platform/auth"
	"github.com/trcmkr/hse-business-club/back/internal/users"
	hbcv1 "github.com/trcmkr/hse-business-club/back/pkg/api/hbc/v1"
)

// Deps are the ports the BFF depends on.
type Deps struct {
	Users        users.Service
	Listings     listings.Service
	Applications applications.Service
	Issuer       auth.Issuer
	// AccessTTL is the lifetime of tokens from Issuer (access_expires_in).
	AccessTTL time.Duration
}

// Implementation implements hbcv1.HseBusinessClubServer.
type Implementation struct {
	hbcv1.UnimplementedHseBusinessClubServer

	Users        users.Service
	Listings     listings.Service
	Applications applications.Service
	Issuer       auth.Issuer
	AccessTTL    time.Duration
}

var _ hbcv1.HseBusinessClubServer = (*Implementation)(nil)

// New creates the BFF.
func New(d Deps) *Implementation {
	return &Implementation{
		Users:        d.Users,
		Listings:     d.Listings,
		Applications: d.Applications,
		Issuer:       d.Issuer,
		AccessTTL:    d.AccessTTL,
	}
}

// publicMethods do not require a token (SPEC §4.1, "все").
var publicMethods = []string{
	hbcv1.HseBusinessClub_V1Ping_FullMethodName,
	hbcv1.HseBusinessClub_V1Register_FullMethodName,
	hbcv1.HseBusinessClub_V1Login_FullMethodName,
	hbcv1.HseBusinessClub_V1Refresh_FullMethodName,
	hbcv1.HseBusinessClub_V1Logout_FullMethodName,
	hbcv1.HseBusinessClub_V1GetAvatar_FullMethodName,
}

// PublicMethods returns full method names that the auth interceptor lets through
// without a token.
func (i *Implementation) PublicMethods() []string {
	return append([]string(nil), publicMethods...)
}

// SwaggerJSON returns the OpenAPI document of hbc.v1.
func (i *Implementation) SwaggerJSON() []byte {
	return hbcv1.SwaggerJSON()
}

// RegisterGRPC registers the service on the gRPC server.
func (i *Implementation) RegisterGRPC(s grpc.ServiceRegistrar) {
	hbcv1.RegisterHseBusinessClubServer(s, i)
}

// RegisterGateway registers HTTP handlers that proxy to the gRPC server over conn.
func (i *Implementation) RegisterGateway(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
	return hbcv1.RegisterHseBusinessClubHandler(ctx, mux, conn)
}

// V1Ping is a liveness probe of the API.
func (i *Implementation) V1Ping(context.Context, *hbcv1.V1PingRequest) (*hbcv1.V1PingResponse, error) {
	return &hbcv1.V1PingResponse{Ok: true}, nil
}
