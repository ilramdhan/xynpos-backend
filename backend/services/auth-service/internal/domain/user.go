package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Entities
// ──────────────────────────────────────────────

// User represents a system user (cross-tenant, global schema).
type User struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email           string     `gorm:"uniqueIndex;not null"`
	Phone           string     `gorm:"index"`
	PasswordHash    string     `gorm:"not null"`
	Name            string     `gorm:"not null"`
	AvatarURL       string
	GoogleID        string     `gorm:"uniqueIndex"`
	IsActive        bool       `gorm:"default:true;not null"`
	EmailVerified   bool       `gorm:"default:false;not null"`
	EmailVerifiedAt *time.Time
	LastLoginAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time `gorm:"index"`
}

// RefreshToken stores a hashed refresh token for rotation + reuse detection.
type RefreshToken struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null;index"`
	TenantID    uuid.UUID  `gorm:"type:uuid;not null;index"`
	TokenHash   string     `gorm:"uniqueIndex;not null"`        // SHA-256 of the raw token
	TokenFamily uuid.UUID  `gorm:"type:uuid;not null;index"`    // For reuse detection
	DeviceID    string
	DeviceName  string
	IPAddress   string
	IsRevoked   bool       `gorm:"default:false;not null"`
	ExpiresAt   time.Time  `gorm:"not null;index"`
	CreatedAt   time.Time
}

// OTP is a one-time password for email verification or password reset.
type OTP struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Type      OTPType   `gorm:"not null"`   // email_verification | password_reset
	Code      string    `gorm:"not null"`
	IsUsed    bool      `gorm:"default:false;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

// OTPType defines the purpose of an OTP.
type OTPType string

const (
	OTPTypeEmailVerification OTPType = "email_verification"
	OTPTypePasswordReset     OTPType = "password_reset"
)

// ──────────────────────────────────────────────
// Value Objects / DTOs
// ──────────────────────────────────────────────

// RegisterInput holds the data needed to register a new tenant + owner.
type RegisterInput struct {
	Name         string `json:"name" validate:"required,min=2,max=100,xss"`
	Email        string `json:"email" validate:"required,email"`
	Password     string `json:"password" validate:"required,min=8,max=72"`
	Phone        string `json:"phone" validate:"omitempty,phone_id"`
	BusinessName string `json:"business_name" validate:"required,min=2,max=200,xss"`
	BusinessType string `json:"business_type" validate:"required,oneof=retail fnb service cafe restaurant general"`
}

// LoginInput holds credentials for login.
type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	DeviceID string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

// TokenPair is the response to a successful login or token refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
	TokenType    string `json:"token_type"` // "Bearer"
}

// RefreshInput holds the refresh token for rotation.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ForgotPasswordInput holds the email to send reset link.
type ForgotPasswordInput struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordInput holds the new password + OTP.
type ResetPasswordInput struct {
	Email       string `json:"email" validate:"required,email"`
	OTP         string `json:"otp" validate:"required,len=6"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}

// VerifyEmailInput holds the OTP for email verification.
type VerifyEmailInput struct {
	OTP string `json:"otp" validate:"required,len=6"`
}

// ResendOTPInput holds the email and OTP type.
type ResendOTPInput struct {
	Email string  `json:"email" validate:"required,email"`
	Type  OTPType `json:"type" validate:"required,oneof=email_verification password_reset"`
}

// ChangePasswordInput holds old + new password.
type ChangePasswordInput struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}

// UpdateProfileInput holds profile update fields.
type UpdateProfileInput struct {
	Name      string `json:"name" validate:"omitempty,min=2,max=100,xss"`
	Phone     string `json:"phone" validate:"omitempty,phone_id"`
	AvatarURL string `json:"avatar_url" validate:"omitempty,url"`
}

// Session represents an active user session (for listing sessions).
type Session struct {
	ID          uuid.UUID `json:"id"`
	DeviceID    string    `json:"device_id"`
	DeviceName  string    `json:"device_name"`
	IPAddress   string    `json:"ip_address"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	IsCurrent   bool      `json:"is_current"`
}

// ──────────────────────────────────────────────
// Domain Errors
// ──────────────────────────────────────────────

var (
	ErrEmailAlreadyExists    = errors.New("email already registered")
	ErrPhoneAlreadyExists    = errors.New("phone number already registered")
	ErrInvalidCredentials    = errors.New("invalid email or password")
	ErrAccountSuspended      = errors.New("account is suspended")
	ErrAccountNotFound       = errors.New("account not found")
	ErrRefreshTokenNotFound  = errors.New("refresh token not found")
	ErrRefreshTokenExpired   = errors.New("refresh token has expired")
	ErrRefreshTokenRevoked   = errors.New("refresh token has been revoked")
	ErrRefreshTokenReused    = errors.New("refresh token reuse detected — all sessions revoked")
	ErrOTPNotFound           = errors.New("OTP not found or expired")
	ErrOTPAlreadyUsed        = errors.New("OTP has already been used")
	ErrEmailNotVerified      = errors.New("please verify your email first")
	ErrSamePassword          = errors.New("new password must be different from old password")
	ErrInvalidOldPassword    = errors.New("current password is incorrect")
	ErrTooManyOTPRequests    = errors.New("too many OTP requests, please wait before requesting again")
)
