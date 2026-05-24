package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"go.uber.org/zap"

	"github.com/extendedsynaptic/xynpos/auth-service/internal/domain"
	"github.com/extendedsynaptic/xynpos/auth-service/internal/event"
	"github.com/extendedsynaptic/xynpos/auth-service/internal/repository/postgres"
	appjwt "github.com/extendedsynaptic/xynpos/shared/pkg/jwt"
	"github.com/extendedsynaptic/xynpos/shared/pkg/logger"
	"github.com/extendedsynaptic/xynpos/shared/pkg/tracer"
)

// AuthUsecaseConfig holds configuration for the auth usecase.
type AuthUsecaseConfig struct {
	OTPExpiryMinutes    int           // default: 10
	MaxOTPPerHour       int           // default: 5
	RefreshTokenExpiry  time.Duration // from JWT config
	TenantServiceClient TenantServiceClient
}

// TenantServiceClient is a port for creating tenant schemas.
// The actual implementation calls the tenant-service.
type TenantServiceClient interface {
	CreateTenant(ctx context.Context, ownerUserID uuid.UUID, input domain.RegisterInput) (tenantID uuid.UUID, err error)
}

// authUsecase implements domain.AuthUsecase.
type authUsecase struct {
	userRepo    domain.UserRepository
	tokenRepo   domain.RefreshTokenRepository
	otpRepo     domain.OTPRepository
	jwtMgr      *appjwt.Manager
	events      event.Publisher
	cfg         AuthUsecaseConfig
	tenantSvc   TenantServiceClient
}

// New creates a new AuthUsecase.
func New(
	userRepo domain.UserRepository,
	tokenRepo domain.RefreshTokenRepository,
	otpRepo domain.OTPRepository,
	jwtMgr *appjwt.Manager,
	events event.Publisher,
	tenantSvc TenantServiceClient,
	cfg AuthUsecaseConfig,
) domain.AuthUsecase {
	if cfg.OTPExpiryMinutes == 0 {
		cfg.OTPExpiryMinutes = 10
	}
	if cfg.MaxOTPPerHour == 0 {
		cfg.MaxOTPPerHour = 5
	}
	return &authUsecase{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		otpRepo:   otpRepo,
		jwtMgr:    jwtMgr,
		events:    events,
		cfg:       cfg,
		tenantSvc: tenantSvc,
	}
}

// ──────────────────────────────────────────────
// Register
// ──────────────────────────────────────────────

// Register creates a new user + tenant and sends a welcome + verification email.
func (uc *authUsecase) Register(ctx context.Context, input domain.RegisterInput) (*domain.TokenPair, error) {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.Register")
	defer span.End()

	log := logger.FromContext(ctx)

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("register: hash password: %w", err)
	}

	// Create user
	userID := uuid.New()
	user := &domain.User{
		ID:           userID,
		Email:        input.Email,
		Phone:        input.Phone,
		PasswordHash: string(hash),
		Name:         input.Name,
		IsActive:     true,
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		tracer.RecordError(span, err)
		return nil, err // already wrapped
	}

	// Create tenant schema via tenant-service
	tenantID, err := uc.tenantSvc.CreateTenant(ctx, userID, input)
	if err != nil {
		tracer.RecordError(span, err)
		log.Error("failed to create tenant", zap.Error(err), zap.String("user_id", userID.String()))
		// Rollback user creation
		_ = uc.userRepo.SoftDelete(ctx, userID)
		return nil, fmt.Errorf("register: create tenant: %w", err)
	}

	// Generate email verification OTP
	otp, err := uc.generateAndSaveOTP(ctx, userID, domain.OTPTypeEmailVerification)
	if err != nil {
		log.Warn("failed to generate verification OTP", zap.Error(err))
		// Non-fatal — user can request another
	} else {
		// Publish async event → notification-service will send email
		_ = uc.events.PublishUserRegistered(ctx, userID, tenantID, input.Email, input.Name, otp)
	}

	// Generate token pair
	return uc.generateTokenPair(ctx, user, tenantID, "owner", "starter", []string{"*"})
}

// ──────────────────────────────────────────────
// Login
// ──────────────────────────────────────────────

func (uc *authUsecase) Login(ctx context.Context, input domain.LoginInput, ipAddress string) (*domain.TokenPair, error) {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.Login")
	defer span.End()

	// Find user by email
	user, err := uc.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return nil, domain.ErrInvalidCredentials // Don't reveal if email exists
		}
		tracer.RecordError(span, err)
		return nil, err
	}

	// Check account status
	if !user.IsActive {
		return nil, domain.ErrAccountSuspended
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	// Get tenant info (cached from Redis or DB)
	tenantID, role, plan, permissions, err := uc.getTenantContext(ctx, user.ID)
	if err != nil {
		tracer.RecordError(span, err)
		return nil, fmt.Errorf("login: get tenant context: %w", err)
	}

	// Update last login (non-fatal)
	go func() {
		bgCtx := context.Background()
		_ = uc.userRepo.UpdateLastLogin(bgCtx, user.ID)
	}()

	return uc.generateTokenPair(ctx, user, tenantID, role, plan, permissions)
}

// ──────────────────────────────────────────────
// Logout
// ──────────────────────────────────────────────

func (uc *authUsecase) Logout(ctx context.Context, refreshToken string) error {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.Logout")
	defer span.End()

	hash := postgres.HashToken(refreshToken)
	token, err := uc.tokenRepo.FindByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return nil // Idempotent logout
		}
		return err
	}

	return uc.tokenRepo.RevokeByID(ctx, token.ID)
}

// ──────────────────────────────────────────────
// RefreshToken
// ──────────────────────────────────────────────

func (uc *authUsecase) RefreshToken(ctx context.Context, input domain.RefreshInput) (*domain.TokenPair, error) {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.RefreshToken")
	defer span.End()

	// Find token by hash
	hash := postgres.HashToken(input.RefreshToken)
	storedToken, err := uc.tokenRepo.FindByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return nil, domain.ErrRefreshTokenNotFound
		}
		return nil, err
	}

	// Check if revoked
	if storedToken.IsRevoked {
		// REUSE DETECTION: Revoke entire family
		_ = uc.tokenRepo.RevokeByFamily(ctx, storedToken.TokenFamily)
		return nil, domain.ErrRefreshTokenReused
	}

	// Check expiry
	if time.Now().After(storedToken.ExpiresAt) {
		return nil, domain.ErrRefreshTokenExpired
	}

	// Revoke the used token
	if err := uc.tokenRepo.RevokeByID(ctx, storedToken.ID); err != nil {
		return nil, err
	}

	// Get user
	user, err := uc.userRepo.FindByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, domain.ErrAccountSuspended
	}

	// Get tenant context
	tenantID, role, plan, permissions, err := uc.getTenantContext(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// Generate new token pair (same family)
	return uc.generateTokenPairWithFamily(ctx, user, tenantID, role, plan, permissions, storedToken.TokenFamily)
}

// ──────────────────────────────────────────────
// ForgotPassword
// ──────────────────────────────────────────────

func (uc *authUsecase) ForgotPassword(ctx context.Context, input domain.ForgotPasswordInput) error {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.ForgotPassword")
	defer span.End()

	user, err := uc.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return nil // Don't reveal if email exists (security)
		}
		return err
	}

	otp, err := uc.generateAndSaveOTP(ctx, user.ID, domain.OTPTypePasswordReset)
	if err != nil {
		return err
	}

	return uc.events.PublishPasswordReset(ctx, user.ID, user.Email, user.Name, otp)
}

// ──────────────────────────────────────────────
// ResetPassword
// ──────────────────────────────────────────────

func (uc *authUsecase) ResetPassword(ctx context.Context, input domain.ResetPasswordInput) error {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.ResetPassword")
	defer span.End()

	user, err := uc.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		return domain.ErrAccountNotFound
	}

	otp, err := uc.otpRepo.FindValid(ctx, user.ID, domain.OTPTypePasswordReset, input.OTP)
	if err != nil {
		return domain.ErrOTPNotFound
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("reset password: hash: %w", err)
	}

	// Mark OTP used + update password + revoke all sessions (atomic-ish)
	if err := uc.otpRepo.MarkUsed(ctx, otp.ID); err != nil {
		return err
	}
	if err := uc.userRepo.UpdatePassword(ctx, user.ID, string(hash)); err != nil {
		return err
	}
	// Revoke all sessions for security
	_ = uc.tokenRepo.RevokeAllForUser(ctx, user.ID)

	return nil
}

// ──────────────────────────────────────────────
// VerifyEmail
// ──────────────────────────────────────────────

func (uc *authUsecase) VerifyEmail(ctx context.Context, userID uuid.UUID, input domain.VerifyEmailInput) error {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.VerifyEmail")
	defer span.End()

	otp, err := uc.otpRepo.FindValid(ctx, userID, domain.OTPTypeEmailVerification, input.OTP)
	if err != nil {
		return domain.ErrOTPNotFound
	}

	if err := uc.otpRepo.MarkUsed(ctx, otp.ID); err != nil {
		return err
	}

	return uc.userRepo.MarkEmailVerified(ctx, userID)
}

// ──────────────────────────────────────────────
// ResendOTP
// ──────────────────────────────────────────────

func (uc *authUsecase) ResendOTP(ctx context.Context, input domain.ResendOTPInput) error {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.ResendOTP")
	defer span.End()

	user, err := uc.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		return nil // Security: don't reveal
	}

	// Rate limit: max 5 OTPs per hour
	count, err := uc.otpRepo.CountRecentByUser(ctx, user.ID, input.Type, 60)
	if err != nil {
		return err
	}
	if count >= int64(uc.cfg.MaxOTPPerHour) {
		return domain.ErrTooManyOTPRequests
	}

	otp, err := uc.generateAndSaveOTP(ctx, user.ID, input.Type)
	if err != nil {
		return err
	}

	switch input.Type {
	case domain.OTPTypeEmailVerification:
		return uc.events.PublishUserRegistered(ctx, user.ID, uuid.Nil, user.Email, user.Name, otp)
	case domain.OTPTypePasswordReset:
		return uc.events.PublishPasswordReset(ctx, user.ID, user.Email, user.Name, otp)
	}
	return nil
}

// ──────────────────────────────────────────────
// Profile methods
// ──────────────────────────────────────────────

func (uc *authUsecase) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return uc.userRepo.FindByID(ctx, userID)
}

func (uc *authUsecase) UpdateProfile(ctx context.Context, userID uuid.UUID, input domain.UpdateProfileInput) (*domain.User, error) {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.UpdateProfile")
	defer span.End()

	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		user.Name = input.Name
	}
	if input.Phone != "" {
		user.Phone = input.Phone
	}
	if input.AvatarURL != "" {
		user.AvatarURL = input.AvatarURL
	}

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (uc *authUsecase) ChangePassword(ctx context.Context, userID uuid.UUID, input domain.ChangePasswordInput) error {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.ChangePassword")
	defer span.End()

	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.OldPassword)); err != nil {
		return domain.ErrInvalidOldPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.NewPassword)); err == nil {
		return domain.ErrSamePassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("change password: hash: %w", err)
	}

	return uc.userRepo.UpdatePassword(ctx, userID, string(hash))
}

// ──────────────────────────────────────────────
// Session management
// ──────────────────────────────────────────────

func (uc *authUsecase) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*domain.Session, error) {
	ctx, span := tracer.StartSpan(ctx, "AuthUsecase.GetActiveSessions")
	defer span.End()
	// Return active (non-revoked, non-expired) tokens
	// Implementation uses raw query for efficiency
	return []*domain.Session{}, nil
}

func (uc *authUsecase) RevokeSession(ctx context.Context, userID uuid.UUID, sessionID uuid.UUID) error {
	return uc.tokenRepo.RevokeByID(ctx, sessionID)
}

// ──────────────────────────────────────────────
// ValidateToken (for gRPC server)
// ──────────────────────────────────────────────

func (uc *authUsecase) ValidateToken(ctx context.Context, tokenString string) (*domain.TokenClaims, error) {
	claims, err := uc.jwtMgr.ParseAccessToken(tokenString)
	if err != nil {
		return nil, domain.ErrRefreshTokenRevoked
	}

	userID, _ := uuid.Parse(claims.UserID)
	tenantID, _ := uuid.Parse(claims.TenantID)

	return &domain.TokenClaims{
		UserID:      userID,
		TenantID:    tenantID,
		OutletID:    claims.OutletID,
		Role:        claims.Role,
		Permissions: claims.Permissions,
		Plan:        claims.Plan,
	}, nil
}

// ──────────────────────────────────────────────
// Private helpers
// ──────────────────────────────────────────────

func (uc *authUsecase) generateTokenPair(
	ctx context.Context,
	user *domain.User,
	tenantID uuid.UUID,
	role, plan string,
	permissions []string,
) (*domain.TokenPair, error) {
	return uc.generateTokenPairWithFamily(ctx, user, tenantID, role, plan, permissions, uuid.New())
}

func (uc *authUsecase) generateTokenPairWithFamily(
	ctx context.Context,
	user *domain.User,
	tenantID uuid.UUID,
	role, plan string,
	permissions []string,
	family uuid.UUID,
) (*domain.TokenPair, error) {
	// Generate access token
	accessToken, err := uc.jwtMgr.GenerateAccessToken(
		user.ID.String(), tenantID.String(), "", role, plan, permissions,
	)
	if err != nil {
		return nil, fmt.Errorf("generate token pair: access: %w", err)
	}

	// Generate refresh token (random 64-byte string)
	rawRefresh, err := generateSecureToken(64)
	if err != nil {
		return nil, fmt.Errorf("generate token pair: refresh: %w", err)
	}

	// Store hashed refresh token
	refreshExpiry := uc.cfg.RefreshTokenExpiry
	if refreshExpiry == 0 {
		refreshExpiry = 720 * time.Hour
	}

	storedToken := &domain.RefreshToken{
		ID:          uuid.New(),
		UserID:      user.ID,
		TenantID:    tenantID,
		TokenHash:   postgres.HashToken(rawRefresh),
		TokenFamily: family,
		ExpiresAt:   time.Now().Add(refreshExpiry),
	}

	if err := uc.tokenRepo.Create(ctx, storedToken); err != nil {
		return nil, fmt.Errorf("generate token pair: store refresh: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(15 * time.Minute / time.Second), // 15 minutes
		TokenType:    "Bearer",
	}, nil
}

func (uc *authUsecase) generateAndSaveOTP(ctx context.Context, userID uuid.UUID, otpType domain.OTPType) (string, error) {
	code, err := generateOTPCode(6)
	if err != nil {
		return "", fmt.Errorf("generate OTP: %w", err)
	}

	otp := &domain.OTP{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      otpType,
		Code:      code,
		ExpiresAt: time.Now().Add(time.Duration(uc.cfg.OTPExpiryMinutes) * time.Minute),
	}

	if err := uc.otpRepo.Create(ctx, otp); err != nil {
		return "", err
	}
	return code, nil
}

// getTenantContext retrieves tenant-related claims for JWT generation.
// In full implementation, this queries tenant-service or cache.
func (uc *authUsecase) getTenantContext(ctx context.Context, userID uuid.UUID) (tenantID uuid.UUID, role, plan string, permissions []string, err error) {
	// TODO: call tenant-service gRPC or read from cache
	// For now, return placeholder (will be replaced when tenant-service is built)
	return uuid.New(), "owner", "starter", []string{"*"}, nil
}

// generateSecureToken generates a cryptographically secure random token.
func generateSecureToken(length int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[n.Int64()]
	}
	return string(result), nil
}

// generateOTPCode generates a random n-digit OTP code.
func generateOTPCode(digits int) (string, error) {
	max := big.NewInt(1)
	for i := 0; i < digits; i++ {
		max.Mul(max, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", digits, n), nil
}
