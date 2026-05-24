package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/extendedsynaptic/xynpos/auth-service/internal/domain"
	appdb "github.com/extendedsynaptic/xynpos/shared/pkg/database"
	apperrors "github.com/extendedsynaptic/xynpos/shared/pkg/errors"
)

// userRepository implements domain.UserRepository using GORM + PostgreSQL.
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository.
// db should be scoped to the global schema (public_xyn).
func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *User) error {
	result := r.db.WithContext(ctx).Create(user)
	if result.Error != nil {
		if isDuplicateEmail(result.Error) {
			return domain.ErrEmailAlreadyExists
		}
		if isDuplicatePhone(result.Error) {
			return domain.ErrPhoneAlreadyExists
		}
		return apperrors.Wrap(result.Error, apperrors.ErrInternal.Code, "failed to create user", 500)
	}
	return nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	result := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, apperrors.Wrap(result.Error, "DB_ERROR", "failed to find user by id", 500)
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	result := r.db.WithContext(ctx).
		Where("LOWER(email) = LOWER(?) AND deleted_at IS NULL", email).
		First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, apperrors.Wrap(result.Error, "DB_ERROR", "failed to find user by email", 500)
	}
	return &user, nil
}

func (r *userRepository) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	var user domain.User
	result := r.db.WithContext(ctx).
		Where("phone = ? AND deleted_at IS NULL", phone).
		First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, apperrors.Wrap(result.Error, "DB_ERROR", "failed to find user by phone", 500)
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	result := r.db.WithContext(ctx).Save(user)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to update user", 500)
	}
	return nil
}

func (r *userRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	result := r.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", userID).
		Update("password_hash", passwordHash)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to update password", 500)
	}
	return nil
}

func (r *userRepository) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"email_verified":    true,
			"email_verified_at": now,
		})
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to mark email verified", 500)
	}
	return nil
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", userID).
		Update("last_login_at", now)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to update last login", 500)
	}
	return nil
}

func (r *userRepository) SoftDelete(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", userID).
		Update("deleted_at", now)
	if result.Error != nil {
		return apperrors.Wrap(result.Error, "DB_ERROR", "failed to soft delete user", 500)
	}
	return nil
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

// isDuplicateEmail detects duplicate email constraint violations.
func isDuplicateEmail(err error) bool {
	return err != nil && strings.Contains(err.Error(), "users_email_key")
}

// isDuplicatePhone detects duplicate phone constraint violations.
func isDuplicatePhone(err error) bool {
	return err != nil && strings.Contains(err.Error(), "users_phone_key")
}

// User is an alias to ensure GORM uses the correct table name.
// The actual struct is domain.User but we define TableName here via a method.
type User = domain.User

// Ensure the GORM model uses the global schema.
func init() {
	// Table name is "users" and search_path is set to public_xyn via middleware
	_ = appdb.TenantSchemaName // import to avoid unused import
}
