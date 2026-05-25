//go:build !cilint

package grpc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	grpcdelivery "github.com/extendedsynaptic/xynpos/auth-service/internal/delivery/grpc"
	"github.com/extendedsynaptic/xynpos/auth-service/internal/domain"
	authpb "github.com/extendedsynaptic/xynpos/shared/proto/auth"
)

// ──────────────────────────────────────────────
// Mock AuthUsecase
// ──────────────────────────────────────────────

type mockAuthUsecase struct{ mock.Mock }

func (m *mockAuthUsecase) Register(ctx context.Context, input domain.RegisterInput) (*domain.TokenPair, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenPair), args.Error(1)
}
func (m *mockAuthUsecase) Login(ctx context.Context, input domain.LoginInput, ip string) (*domain.TokenPair, error) {
	args := m.Called(ctx, input, ip)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenPair), args.Error(1)
}
func (m *mockAuthUsecase) Logout(ctx context.Context, refreshToken string) error {
	return m.Called(ctx, refreshToken).Error(0)
}
func (m *mockAuthUsecase) RefreshToken(ctx context.Context, input domain.RefreshInput) (*domain.TokenPair, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenPair), args.Error(1)
}
func (m *mockAuthUsecase) ForgotPassword(ctx context.Context, input domain.ForgotPasswordInput) error {
	return m.Called(ctx, input).Error(0)
}
func (m *mockAuthUsecase) ResetPassword(ctx context.Context, input domain.ResetPasswordInput) error {
	return m.Called(ctx, input).Error(0)
}
func (m *mockAuthUsecase) VerifyEmail(ctx context.Context, userID uuid.UUID, input domain.VerifyEmailInput) error {
	return m.Called(ctx, userID, input).Error(0)
}
func (m *mockAuthUsecase) ResendOTP(ctx context.Context, input domain.ResendOTPInput) error {
	return m.Called(ctx, input).Error(0)
}
func (m *mockAuthUsecase) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *mockAuthUsecase) UpdateProfile(ctx context.Context, userID uuid.UUID, input domain.UpdateProfileInput) (*domain.User, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *mockAuthUsecase) ChangePassword(ctx context.Context, userID uuid.UUID, input domain.ChangePasswordInput) error {
	return m.Called(ctx, userID, input).Error(0)
}
func (m *mockAuthUsecase) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*domain.Session, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Session), args.Error(1)
}
func (m *mockAuthUsecase) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	return m.Called(ctx, userID, sessionID).Error(0)
}
func (m *mockAuthUsecase) ValidateToken(ctx context.Context, accessToken string) (*domain.TokenClaims, error) {
	args := m.Called(ctx, accessToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenClaims), args.Error(1)
}

// ──────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────

func TestValidateToken_ValidToken(t *testing.T) {
	uc := &mockAuthUsecase{}
	srv := grpcdelivery.NewAuthServer(uc, zap.NewNop())

	userID := uuid.New()
	tenantID := uuid.New()

	uc.On("ValidateToken", mock.Anything, "valid.jwt.token").Return(&domain.TokenClaims{
		UserID:      userID,
		TenantID:    tenantID,
		Role:        "owner",
		Plan:        "pro",
		Permissions: []string{"product:*", "transaction:*"},
	}, nil)

	resp, err := srv.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{
		Token: "valid.jwt.token",
	})

	require.NoError(t, err)
	assert.True(t, resp.Valid)
	assert.Equal(t, userID.String(), resp.UserId)
	assert.Equal(t, tenantID.String(), resp.TenantId)
	assert.Equal(t, "owner", resp.Role)
	assert.Equal(t, "pro", resp.Plan)
	assert.Contains(t, resp.Permissions, "product:*")
}

func TestValidateToken_InvalidToken(t *testing.T) {
	uc := &mockAuthUsecase{}
	srv := grpcdelivery.NewAuthServer(uc, zap.NewNop())

	uc.On("ValidateToken", mock.Anything, "invalid.token").Return(nil, domain.ErrRefreshTokenRevoked)

	resp, err := srv.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{
		Token: "invalid.token",
	})

	require.NoError(t, err) // gRPC should not return error, but valid=false
	assert.False(t, resp.Valid)
	assert.NotEmpty(t, resp.Error)
}

func TestValidateToken_EmptyToken(t *testing.T) {
	uc := &mockAuthUsecase{}
	srv := grpcdelivery.NewAuthServer(uc, zap.NewNop())

	resp, err := srv.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{
		Token: "",
	})

	require.NoError(t, err)
	assert.False(t, resp.Valid)
	assert.Equal(t, "token is required", resp.Error)
	uc.AssertNotCalled(t, "ValidateToken")
}

func TestGetUserPermissions_MissingArgs(t *testing.T) {
	uc := &mockAuthUsecase{}
	srv := grpcdelivery.NewAuthServer(uc, zap.NewNop())

	_, err := srv.GetUserPermissions(context.Background(), &authpb.GetPermissionsRequest{
		UserId:   "",
		TenantId: "",
	})

	assert.Error(t, err)
}

func TestGetUserPermissions_Success(t *testing.T) {
	uc := &mockAuthUsecase{}
	srv := grpcdelivery.NewAuthServer(uc, zap.NewNop())

	resp, err := srv.GetUserPermissions(context.Background(), &authpb.GetPermissionsRequest{
		UserId:   uuid.New().String(),
		TenantId: uuid.New().String(),
	})

	require.NoError(t, err)
	assert.Equal(t, "see_token", resp.Role)
}
