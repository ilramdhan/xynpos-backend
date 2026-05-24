package domain

import (
	"context"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Repository Interfaces
// ──────────────────────────────────────────────

// UserRepository defines persistence operations for User.
// All implementations must respect context cancellation and deadlines.
type UserRepository interface {
	// Create persists a new user. Returns ErrEmailAlreadyExists if email is taken.
	Create(ctx context.Context, user *User) error

	// FindByID retrieves a user by primary key. Returns ErrAccountNotFound if missing.
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)

	// FindByEmail retrieves a user by email (case-insensitive).
	FindByEmail(ctx context.Context, email string) (*User, error)

	// FindByPhone retrieves a user by phone number.
	FindByPhone(ctx context.Context, phone string) (*User, error)

	// Update updates mutable user fields.
	Update(ctx context.Context, user *User) error

	// UpdatePassword updates the password hash for a user.
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error

	// MarkEmailVerified marks a user's email as verified.
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error

	// UpdateLastLogin updates the last login timestamp.
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error

	// SoftDelete soft-deletes a user.
	SoftDelete(ctx context.Context, userID uuid.UUID) error
}

// RefreshTokenRepository defines persistence for refresh tokens.
type RefreshTokenRepository interface {
	// Create stores a new refresh token.
	Create(ctx context.Context, token *RefreshToken) error

	// FindByHash retrieves a token by its SHA-256 hash.
	FindByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)

	// FindByFamily retrieves all tokens in a token family (for reuse detection).
	FindByFamily(ctx context.Context, family uuid.UUID) ([]*RefreshToken, error)

	// RevokeByID marks a single refresh token as revoked.
	RevokeByID(ctx context.Context, tokenID uuid.UUID) error

	// RevokeByFamily marks all tokens in a family as revoked (reuse attack response).
	RevokeByFamily(ctx context.Context, family uuid.UUID) error

	// RevokeAllForUser revokes all refresh tokens for a user (logout all).
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error

	// DeleteExpired deletes all expired tokens (cron cleanup).
	DeleteExpired(ctx context.Context) (int64, error)
}

// OTPRepository defines persistence for one-time passwords.
type OTPRepository interface {
	// Create creates a new OTP (invalidates previous OTPs of the same type for user).
	Create(ctx context.Context, otp *OTP) error

	// FindValid retrieves a valid (unused, non-expired) OTP for a user+type+code.
	FindValid(ctx context.Context, userID uuid.UUID, otpType OTPType, code string) (*OTP, error)

	// MarkUsed marks an OTP as used.
	MarkUsed(ctx context.Context, otpID uuid.UUID) error

	// CountRecentByUser counts OTPs created for a user in the last N minutes (rate limiting).
	CountRecentByUser(ctx context.Context, userID uuid.UUID, otpType OTPType, minutes int) (int64, error)

	// DeleteExpired deletes all expired OTPs (cron cleanup).
	DeleteExpired(ctx context.Context) (int64, error)
}

// ──────────────────────────────────────────────
// Usecase Interface
// ──────────────────────────────────────────────

// AuthUsecase defines all business operations for the auth service.
type AuthUsecase interface {
	// Register creates a new tenant + owner user account.
	Register(ctx context.Context, input RegisterInput) (*TokenPair, error)

	// Login authenticates a user and returns a token pair.
	Login(ctx context.Context, input LoginInput, ipAddress string) (*TokenPair, error)

	// Logout revokes the user's current refresh token.
	Logout(ctx context.Context, refreshToken string) error

	// RefreshToken rotates the refresh token and returns a new token pair.
	RefreshToken(ctx context.Context, input RefreshInput) (*TokenPair, error)

	// ForgotPassword sends a password reset OTP to the user's email.
	ForgotPassword(ctx context.Context, input ForgotPasswordInput) error

	// ResetPassword resets the password using a valid OTP.
	ResetPassword(ctx context.Context, input ResetPasswordInput) error

	// VerifyEmail verifies the user's email using a 6-digit OTP.
	VerifyEmail(ctx context.Context, userID uuid.UUID, input VerifyEmailInput) error

	// ResendOTP sends a new OTP (with rate limiting).
	ResendOTP(ctx context.Context, input ResendOTPInput) error

	// GetProfile returns the authenticated user's profile.
	GetProfile(ctx context.Context, userID uuid.UUID) (*User, error)

	// UpdateProfile updates mutable profile fields.
	UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateProfileInput) (*User, error)

	// ChangePassword changes the password after verifying the old one.
	ChangePassword(ctx context.Context, userID uuid.UUID, input ChangePasswordInput) error

	// GetActiveSessions returns all active refresh tokens for a user.
	GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*Session, error)

	// RevokeSession revokes a specific refresh token by ID.
	RevokeSession(ctx context.Context, userID uuid.UUID, sessionID uuid.UUID) error

	// ValidateToken validates an access token (used by gRPC server).
	ValidateToken(ctx context.Context, tokenString string) (*TokenClaims, error)
}

// TokenClaims is returned by ValidateToken for internal gRPC use.
type TokenClaims struct {
	UserID      uuid.UUID
	TenantID    uuid.UUID
	OutletID    string
	Role        string
	Permissions []string
	Plan        string
}
