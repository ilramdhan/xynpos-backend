package grpc

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/extendedsynaptic/xynpos/auth-service/internal/domain"
	authpb "github.com/extendedsynaptic/xynpos/shared/proto/auth"
)

var tracer = otel.Tracer("auth-service/grpc")

// AuthServer implements the gRPC AuthServiceServer.
// Used for service-to-service internal communication only.
type AuthServer struct {
	authpb.UnimplementedAuthServiceServer
	usecase domain.AuthUsecase
	log     *zap.Logger
}

// NewAuthServer creates a new gRPC AuthServer.
func NewAuthServer(uc domain.AuthUsecase, log *zap.Logger) *AuthServer {
	return &AuthServer{usecase: uc, log: log}
}

// ValidateToken validates an access token and returns embedded claims.
// Called by other microservices (product-service, tenant-service, etc.)
// via the API Gateway or internal service mesh.
func (s *AuthServer) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	ctx, span := tracer.Start(ctx, "AuthServer.ValidateToken")
	defer span.End()

	if req.GetToken() == "" {
		return &authpb.ValidateTokenResponse{
			Valid: false,
			Error: "token is required",
		}, nil
	}

	claims, err := s.usecase.ValidateToken(ctx, req.GetToken())
	if err != nil {
		s.log.Debug("token validation failed", zap.Error(err))
		return &authpb.ValidateTokenResponse{
			Valid: false,
			Error: err.Error(),
		}, nil
	}

	span.SetAttributes(
		attribute.String("user.id", claims.UserID.String()),
		attribute.String("tenant.id", claims.TenantID.String()),
		attribute.String("user.role", claims.Role),
	)

	return &authpb.ValidateTokenResponse{
		Valid:       true,
		UserId:      claims.UserID.String(),
		TenantId:    claims.TenantID.String(),
		OutletId:    claims.OutletID,
		Role:        claims.Role,
		Plan:        claims.Plan,
		Permissions: claims.Permissions,
	}, nil
}

// GetUserPermissions returns permission list for a user in a tenant.
// Used by API Gateway for fine-grained authorization.
func (s *AuthServer) GetUserPermissions(ctx context.Context, req *authpb.GetPermissionsRequest) (*authpb.GetPermissionsResponse, error) {
	ctx, span := tracer.Start(ctx, "AuthServer.GetUserPermissions")
	defer span.End()

	if req.GetUserId() == "" || req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and tenant_id are required")
	}

	// For now, permissions are embedded in the JWT token itself.
	// In the future, this will query a permission-service or RBAC store.
	// Return a signal that the caller should use the token's embedded permissions.
	return &authpb.GetPermissionsResponse{
		Role:        "see_token",
		Permissions: []string{},
	}, nil
}
