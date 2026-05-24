package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/extendedsynaptic/xynpos/auth-service/internal/domain"
	"github.com/extendedsynaptic/xynpos/auth-service/internal/usecase"
	appjwt "github.com/extendedsynaptic/xynpos/shared/pkg/jwt"
)

// ──────────────────────────────────────────────
// Mocks (hand-written for speed; use mockery for complex cases)
// ──────────────────────────────────────────────

type mockUserRepo struct{ mock.Mock }
type mockTokenRepo struct{ mock.Mock }
type mockOTPRepo struct{ mock.Mock }
type mockEventPublisher struct{ mock.Mock }
type mockTenantClient struct{ mock.Mock }

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}
func (m *mockUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *mockUserRepo) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	args := m.Called(ctx, phone)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}
func (m *mockUserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	return m.Called(ctx, id, hash).Error(0)
}
func (m *mockUserRepo) MarkEmailVerified(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockUserRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockUserRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockTokenRepo) Create(ctx context.Context, t *domain.RefreshToken) error {
	return m.Called(ctx, t).Error(0)
}
func (m *mockTokenRepo) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefreshToken), args.Error(1)
}
func (m *mockTokenRepo) FindByFamily(ctx context.Context, family uuid.UUID) ([]*domain.RefreshToken, error) {
	args := m.Called(ctx, family)
	return args.Get(0).([]*domain.RefreshToken), args.Error(1)
}
func (m *mockTokenRepo) RevokeByID(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockTokenRepo) RevokeByFamily(ctx context.Context, family uuid.UUID) error {
	return m.Called(ctx, family).Error(0)
}
func (m *mockTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	return m.Called(ctx, userID).Error(0)
}
func (m *mockTokenRepo) DeleteExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockOTPRepo) Create(ctx context.Context, otp *domain.OTP) error {
	return m.Called(ctx, otp).Error(0)
}
func (m *mockOTPRepo) FindValid(ctx context.Context, userID uuid.UUID, t domain.OTPType, code string) (*domain.OTP, error) {
	args := m.Called(ctx, userID, t, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.OTP), args.Error(1)
}
func (m *mockOTPRepo) MarkUsed(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockOTPRepo) CountRecentByUser(ctx context.Context, userID uuid.UUID, t domain.OTPType, mins int) (int64, error) {
	args := m.Called(ctx, userID, t, mins)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockOTPRepo) DeleteExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockEventPublisher) PublishUserRegistered(ctx context.Context, userID, tenantID uuid.UUID, email, name, otp string) error {
	return m.Called(ctx, userID, tenantID, email, name, otp).Error(0)
}
func (m *mockEventPublisher) PublishPasswordReset(ctx context.Context, userID uuid.UUID, email, name, otp string) error {
	return m.Called(ctx, userID, email, name, otp).Error(0)
}
func (m *mockEventPublisher) PublishPasswordChanged(ctx context.Context, userID uuid.UUID, email string) error {
	return m.Called(ctx, userID, email).Error(0)
}

func (m *mockTenantClient) CreateTenant(ctx context.Context, ownerID uuid.UUID, input domain.RegisterInput) (uuid.UUID, error) {
	args := m.Called(ctx, ownerID, input)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

// ──────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────

func newTestJWTManager() *appjwt.Manager {
	return appjwt.New(appjwt.Config{
		AccessSecret:  "test_access_secret_very_long_32chars!",
		RefreshSecret: "test_refresh_secret_very_long_32ch",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 720 * time.Hour,
		Issuer:        "xynpos.com",
	})
}

func newTestUsecase(
	userRepo domain.UserRepository,
	tokenRepo domain.RefreshTokenRepository,
	otpRepo domain.OTPRepository,
	evts *mockEventPublisher,
	tenantClient *mockTenantClient,
) domain.AuthUsecase {
	return usecase.New(userRepo, tokenRepo, otpRepo, newTestJWTManager(), evts, tenantClient,
		usecase.AuthUsecaseConfig{OTPExpiryMinutes: 10, MaxOTPPerHour: 5, RefreshTokenExpiry: 720 * time.Hour})
}

// ──────────────────────────────────────────────
// Tests: Register
// ──────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)
	tenantID := uuid.New()

	userRepo.On("Create", mock.Anything, mock.MatchedBy(func(u *domain.User) bool {
		return u.Email == "test@example.com"
	})).Return(nil)

	tenantClient.On("CreateTenant", mock.Anything, mock.Anything, mock.Anything).
		Return(tenantID, nil)

	otpRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	evts.On("PublishUserRegistered", mock.Anything, mock.Anything, tenantID, "test@example.com", "Test User", mock.Anything).
		Return(nil)
	tokenRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	input := domain.RegisterInput{
		Name:         "Test User",
		Email:        "test@example.com",
		Password:     "SecurePass123!",
		BusinessName: "Test Business",
		BusinessType: "retail",
	}

	tokens, err := uc.Register(context.Background(), input)
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	assert.Equal(t, "Bearer", tokens.TokenType)

	userRepo.AssertExpectations(t)
	tenantClient.AssertExpectations(t)
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	userRepo.On("Create", mock.Anything, mock.Anything).Return(domain.ErrEmailAlreadyExists)

	input := domain.RegisterInput{
		Name:         "Test User",
		Email:        "existing@example.com",
		Password:     "SecurePass123!",
		BusinessName: "Test Business",
		BusinessType: "retail",
	}

	_, err := uc.Register(context.Background(), input)
	assert.ErrorIs(t, err, domain.ErrEmailAlreadyExists)
}

// ──────────────────────────────────────────────
// Tests: Login
// ──────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := &domain.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: string(hash),
		IsActive:     true,
	}

	userRepo.On("FindByEmail", mock.Anything, "test@example.com").Return(user, nil)
	tokenRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	userRepo.On("UpdateLastLogin", mock.Anything, user.ID).Return(nil).Maybe()

	tokens, err := uc.Login(context.Background(), domain.LoginInput{
		Email:    "test@example.com",
		Password: "password123",
	}, "127.0.0.1")

	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
}

func TestLogin_WrongPassword(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.DefaultCost)
	user := &domain.User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: string(hash),
		IsActive:     true,
	}

	userRepo.On("FindByEmail", mock.Anything, "test@example.com").Return(user, nil)

	_, err := uc.Login(context.Background(), domain.LoginInput{
		Email:    "test@example.com",
		Password: "wrong_password",
	}, "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestLogin_AccountNotFound_ReturnsInvalidCredentials(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	// Email not found → must return ErrInvalidCredentials (not ErrAccountNotFound)
	// This is security requirement — don't leak if email exists
	userRepo.On("FindByEmail", mock.Anything, "notfound@example.com").
		Return(nil, domain.ErrAccountNotFound)

	_, err := uc.Login(context.Background(), domain.LoginInput{
		Email:    "notfound@example.com",
		Password: "anypassword",
	}, "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrInvalidCredentials)
	// Must NOT be ErrAccountNotFound
	assert.False(t, errors.Is(err, domain.ErrAccountNotFound))
}

func TestLogin_SuspendedAccount(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := &domain.User{
		ID:           uuid.New(),
		Email:        "suspended@example.com",
		PasswordHash: string(hash),
		IsActive:     false, // Suspended
	}

	userRepo.On("FindByEmail", mock.Anything, "suspended@example.com").Return(user, nil)

	_, err := uc.Login(context.Background(), domain.LoginInput{
		Email:    "suspended@example.com",
		Password: "password123",
	}, "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrAccountSuspended)
}

// ──────────────────────────────────────────────
// Tests: RefreshToken (Rotation + Reuse Detection)
// ──────────────────────────────────────────────

func TestRefreshToken_Success(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	rawToken := "valid_refresh_token_string"
	family := uuid.New()
	storedToken := &domain.RefreshToken{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		TenantID:    uuid.New(),
		TokenFamily: family,
		IsRevoked:   false,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	user := &domain.User{
		ID:       storedToken.UserID,
		Email:    "test@example.com",
		IsActive: true,
	}

	tokenRepo.On("FindByHash", mock.Anything, mock.Anything).Return(storedToken, nil)
	tokenRepo.On("RevokeByID", mock.Anything, storedToken.ID).Return(nil)
	tokenRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	userRepo.On("FindByID", mock.Anything, storedToken.UserID).Return(user, nil)

	tokens, err := uc.RefreshToken(context.Background(), domain.RefreshInput{RefreshToken: rawToken})
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	// New token must be different from old one
	assert.NotEqual(t, rawToken, tokens.RefreshToken)
}

func TestRefreshToken_ReuseDetection_RevokesEntireFamily(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	family := uuid.New()
	revokedToken := &domain.RefreshToken{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		TokenFamily: family,
		IsRevoked:   true, // Already revoked = REUSE ATTACK
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	tokenRepo.On("FindByHash", mock.Anything, mock.Anything).Return(revokedToken, nil)
	// MUST revoke entire family when reuse is detected
	tokenRepo.On("RevokeByFamily", mock.Anything, family).Return(nil)

	_, err := uc.RefreshToken(context.Background(), domain.RefreshInput{RefreshToken: "stolen_token"})
	assert.ErrorIs(t, err, domain.ErrRefreshTokenReused)
	tokenRepo.AssertCalled(t, "RevokeByFamily", mock.Anything, family)
}

// ──────────────────────────────────────────────
// Tests: VerifyEmail
// ──────────────────────────────────────────────

func TestVerifyEmail_Success(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	userID := uuid.New()
	otp := &domain.OTP{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      domain.OTPTypeEmailVerification,
		Code:      "123456",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	otpRepo.On("FindValid", mock.Anything, userID, domain.OTPTypeEmailVerification, "123456").Return(otp, nil)
	otpRepo.On("MarkUsed", mock.Anything, otp.ID).Return(nil)
	userRepo.On("MarkEmailVerified", mock.Anything, userID).Return(nil)

	err := uc.VerifyEmail(context.Background(), userID, domain.VerifyEmailInput{OTP: "123456"})
	require.NoError(t, err)

	userRepo.AssertCalled(t, "MarkEmailVerified", mock.Anything, userID)
}

func TestVerifyEmail_InvalidOTP(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	userID := uuid.New()
	otpRepo.On("FindValid", mock.Anything, userID, domain.OTPTypeEmailVerification, "999999").
		Return(nil, domain.ErrOTPNotFound)

	err := uc.VerifyEmail(context.Background(), userID, domain.VerifyEmailInput{OTP: "999999"})
	assert.ErrorIs(t, err, domain.ErrOTPNotFound)
}

// ──────────────────────────────────────────────
// Tests: ForgotPassword (email enumeration prevention)
// ──────────────────────────────────────────────

func TestForgotPassword_UnknownEmail_NoError(t *testing.T) {
	userRepo := &mockUserRepo{}
	tokenRepo := &mockTokenRepo{}
	otpRepo := &mockOTPRepo{}
	evts := &mockEventPublisher{}
	tenantClient := &mockTenantClient{}

	uc := newTestUsecase(userRepo, tokenRepo, otpRepo, evts, tenantClient)

	userRepo.On("FindByEmail", mock.Anything, "unknown@example.com").
		Return(nil, domain.ErrAccountNotFound)

	// Must NOT return error — prevents email enumeration
	err := uc.ForgotPassword(context.Background(), domain.ForgotPasswordInput{
		Email: "unknown@example.com",
	})
	assert.NoError(t, err)
}
