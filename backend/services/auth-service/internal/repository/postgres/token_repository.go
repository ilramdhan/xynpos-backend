package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/extendedsynaptic/xynpos/auth-service/internal/domain"
	apperrors "github.com/extendedsynaptic/xynpos/shared/pkg/errors"
)

// refreshTokenRepository implements domain.RefreshTokenRepository.
type refreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository creates a new refresh token repository.
func NewRefreshTokenRepository(db *gorm.DB) domain.RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

func (r *refreshTokenRepository) Create(ctx context.Context, token *domain.RefreshToken) error {
	result := r.db.WithContext(ctx).Create(token)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to create refresh token", 500)
	}
	return nil
}

func (r *refreshTokenRepository) FindByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	var token domain.RefreshToken
	result := r.db.WithContext(ctx).
		Where("token_hash = ?", tokenHash).
		First(&token)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRefreshTokenNotFound
		}
		return nil, apperrors.Wrap(result.Error, "DB_ERROR", "failed to find refresh token", 500)
	}
	return &token, nil
}

func (r *refreshTokenRepository) FindByFamily(ctx context.Context, family uuid.UUID) ([]*domain.RefreshToken, error) {
	var tokens []*domain.RefreshToken
	result := r.db.WithContext(ctx).
		Where("token_family = ?", family).
		Find(&tokens)
	if result.Error != nil {
		return nil, apperrors.Wrap(result.Error, "DB_ERROR", "failed to find token family", 500)
	}
	return tokens, nil
}

func (r *refreshTokenRepository) RevokeByID(ctx context.Context, tokenID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("id = ?", tokenID).
		Update("is_revoked", true)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to revoke token", 500)
	}
	return nil
}

func (r *refreshTokenRepository) RevokeByFamily(ctx context.Context, family uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("token_family = ?", family).
		Update("is_revoked", true)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to revoke token family", 500)
	}
	return nil
}

func (r *refreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&domain.RefreshToken{}).
		Where("user_id = ? AND is_revoked = false", userID).
		Update("is_revoked", true)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to revoke all user tokens", 500)
	}
	return nil
}

func (r *refreshTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&domain.RefreshToken{})
	if result.Error != nil {
		return 0, apperrors.Wrap(result.Error, "DB_ERROR", "failed to delete expired tokens", 500)
	}
	return result.RowsAffected, nil
}

// HashToken creates a SHA-256 hash of a raw token for safe storage.
func HashToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return fmt.Sprintf("%x", h)
}

// ──────────────────────────────────────────────
// OTP Repository
// ──────────────────────────────────────────────

// otpRepository implements domain.OTPRepository.
type otpRepository struct {
	db *gorm.DB
}

// NewOTPRepository creates a new OTP repository.
func NewOTPRepository(db *gorm.DB) domain.OTPRepository {
	return &otpRepository{db: db}
}

func (r *otpRepository) Create(ctx context.Context, otp *domain.OTP) error {
	// Invalidate previous unused OTPs of the same type for this user
	r.db.WithContext(ctx).Model(&domain.OTP{}).
		Where("user_id = ? AND type = ? AND is_used = false", otp.UserID, otp.Type).
		Update("is_used", true)

	result := r.db.WithContext(ctx).Create(otp)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to create OTP", 500)
	}
	return nil
}

func (r *otpRepository) FindValid(ctx context.Context, userID uuid.UUID, otpType domain.OTPType, code string) (*domain.OTP, error) {
	var otp domain.OTP
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND type = ? AND code = ? AND is_used = false AND expires_at > ?",
			userID, otpType, code, time.Now()).
		First(&otp)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOTPNotFound
		}
		return nil, apperrors.Wrap(result.Error, "DB_ERROR", "failed to find OTP", 500)
	}
	return &otp, nil
}

func (r *otpRepository) MarkUsed(ctx context.Context, otpID uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&domain.OTP{}).
		Where("id = ?", otpID).
		Update("is_used", true)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to mark OTP as used", 500)
	}
	return nil
}

func (r *otpRepository) CountRecentByUser(ctx context.Context, userID uuid.UUID, otpType domain.OTPType, minutes int) (int64, error) {
	var count int64
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	result := r.db.WithContext(ctx).Model(&domain.OTP{}).
		Where("user_id = ? AND type = ? AND created_at > ?", userID, otpType, since).
		Count(&count)
	if result.Error != nil {
		return 0, apperrors.Wrap(result.Error, "DB_ERROR", "failed to count OTPs", 500)
	}
	return count, nil
}

func (r *otpRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&domain.OTP{})
	if result.Error != nil {
		return 0, apperrors.Wrap(result.Error, "DB_ERROR", "failed to delete expired OTPs", 500)
	}
	return result.RowsAffected, nil
}
