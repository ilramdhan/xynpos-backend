package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/extendedsynaptic/xynpos/tenant-service/internal/domain"
	apperrors "github.com/extendedsynaptic/xynpos/shared/pkg/errors"
)

// ──────────────────────────────────────────────
// Tenant Repository
// ──────────────────────────────────────────────

type tenantRepo struct{ db *gorm.DB }

// NewTenantRepository creates a new TenantRepository backed by PostgreSQL.
func NewTenantRepository(db *gorm.DB) domain.TenantRepository {
	return &tenantRepo{db: db}
}

func (r *tenantRepo) Create(ctx context.Context, t *domain.Tenant) error {
	if err := r.db.WithContext(ctx).Create(t).Error; err != nil {
		if strings.Contains(err.Error(), "tenants_slug_key") {
			return domain.ErrSlugAlreadyExists
		}
		return apperrors.Wrap(err, "DB_ERROR", "failed to create tenant", 500)
	}
	return nil
}

func (r *tenantRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	var t domain.Tenant
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTenantNotFound
		}
		return nil, apperrors.Wrap(err, "DB_ERROR", "find tenant by id", 500)
	}
	return &t, nil
}

func (r *tenantRepo) FindByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]*domain.Tenant, error) {
	var tenants []*domain.Tenant
	if err := r.db.WithContext(ctx).
		Where("owner_id = ? AND deleted_at IS NULL", ownerID).
		Order("created_at ASC").
		Find(&tenants).Error; err != nil {
		return nil, apperrors.Wrap(err, "DB_ERROR", "find tenants by owner", 500)
	}
	return tenants, nil
}

func (r *tenantRepo) FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	var t domain.Tenant
	if err := r.db.WithContext(ctx).Where("slug = ? AND deleted_at IS NULL", slug).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTenantNotFound
		}
		return nil, apperrors.Wrap(err, "DB_ERROR", "find tenant by slug", 500)
	}
	return &t, nil
}

func (r *tenantRepo) Update(ctx context.Context, t *domain.Tenant) error {
	if err := r.db.WithContext(ctx).Save(t).Error; err != nil {
		return apperrors.Wrap(err, "DB_ERROR", "update tenant", 500)
	}
	return nil
}

func (r *tenantRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.Tenant{}).
		Where("id = ?", id).Update("deleted_at", now).Error
}

func (r *tenantRepo) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Tenant{}).
		Where("id = ? AND deleted_at IS NULL", id).Count(&count).Error
	return count > 0, err
}

// ──────────────────────────────────────────────
// Outlet Repository
// ──────────────────────────────────────────────

type outletRepo struct{ db *gorm.DB }

// NewOutletRepository creates a new OutletRepository backed by PostgreSQL.
func NewOutletRepository(db *gorm.DB) domain.OutletRepository {
	return &outletRepo{db: db}
}

func (r *outletRepo) Create(ctx context.Context, o *domain.Outlet) error {
	if err := r.db.WithContext(ctx).Create(o).Error; err != nil {
		if strings.Contains(err.Error(), "outlets_code") {
			return domain.ErrOutletCodeExists
		}
		return apperrors.Wrap(err, "DB_ERROR", "create outlet", 500)
	}
	return nil
}

func (r *outletRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Outlet, error) {
	var o domain.Outlet
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOutletNotFound
		}
		return nil, apperrors.Wrap(err, "DB_ERROR", "find outlet", 500)
	}
	return &o, nil
}

func (r *outletRepo) FindByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.Outlet, error) {
	var outlets []*domain.Outlet
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("is_main_outlet DESC, name ASC").
		Find(&outlets).Error; err != nil {
		return nil, apperrors.Wrap(err, "DB_ERROR", "find outlets by tenant", 500)
	}
	return outlets, nil
}

func (r *outletRepo) Update(ctx context.Context, o *domain.Outlet) error {
	return r.db.WithContext(ctx).Save(o).Error
}

func (r *outletRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.Outlet{}).
		Where("id = ?", id).Update("deleted_at", now).Error
}

func (r *outletRepo) CountByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Outlet{}).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Count(&count).Error
	return count, err
}

// ──────────────────────────────────────────────
// TenantUser Repository
// ──────────────────────────────────────────────

type tenantUserRepo struct{ db *gorm.DB }

// NewTenantUserRepository creates a new TenantUserRepository backed by PostgreSQL.
func NewTenantUserRepository(db *gorm.DB) domain.TenantUserRepository {
	return &tenantUserRepo{db: db}
}

func (r *tenantUserRepo) Create(ctx context.Context, tu *domain.TenantUser) error {
	if err := r.db.WithContext(ctx).Create(tu).Error; err != nil {
		if strings.Contains(err.Error(), "tenant_users_unique") {
			return domain.ErrAlreadyMember
		}
		return apperrors.Wrap(err, "DB_ERROR", "create tenant user", 500)
	}
	return nil
}

func (r *tenantUserRepo) FindByTenantAndUser(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantUser, error) {
	var tu domain.TenantUser
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND is_active = true", tenantID, userID).
		First(&tu).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotTenantMember
		}
		return nil, apperrors.Wrap(err, "DB_ERROR", "find tenant user", 500)
	}
	return &tu, nil
}

func (r *tenantUserRepo) FindByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantUser, error) {
	var members []*domain.TenantUser
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_active = true", tenantID).
		Find(&members).Error; err != nil {
		return nil, apperrors.Wrap(err, "DB_ERROR", "find tenant members", 500)
	}
	return members, nil
}

func (r *tenantUserRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.TenantUser, error) {
	var tus []*domain.TenantUser
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = true", userID).
		Find(&tus).Error; err != nil {
		return nil, apperrors.Wrap(err, "DB_ERROR", "find user tenants", 500)
	}
	return tus, nil
}

func (r *tenantUserRepo) Update(ctx context.Context, tu *domain.TenantUser) error {
	return r.db.WithContext(ctx).Save(tu).Error
}

func (r *tenantUserRepo) Remove(ctx context.Context, tenantID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&domain.TenantUser{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("is_active", false).Error
}

func (r *tenantUserRepo) CountByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.TenantUser{}).
		Where("tenant_id = ? AND is_active = true", tenantID).Count(&count).Error
	return count, err
}

// ──────────────────────────────────────────────
// Role Repository
// ──────────────────────────────────────────────

type roleRepo struct{ db *gorm.DB }

// NewRoleRepository creates a new RoleRepository backed by PostgreSQL.
func NewRoleRepository(db *gorm.DB) domain.RoleRepository {
	return &roleRepo{db: db}
}

func (r *roleRepo) FindAll(ctx context.Context, tenantID *uuid.UUID) ([]*domain.Role, error) {
	var roles []*domain.Role
	q := r.db.WithContext(ctx).Where("is_active = true")
	if tenantID != nil {
		q = q.Where("tenant_id = ? OR tenant_id IS NULL", tenantID)
	} else {
		q = q.Where("tenant_id IS NULL")
	}
	if err := q.Order("is_system DESC, name ASC").Find(&roles).Error; err != nil {
		return nil, apperrors.Wrap(err, "DB_ERROR", "find roles", 500)
	}
	return roles, nil
}

func (r *roleRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	var role domain.Role
	if err := r.db.WithContext(ctx).Where("id = ? AND is_active = true", id).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, apperrors.Wrap(err, "DB_ERROR", "find role by id", 500)
	}
	return &role, nil
}

func (r *roleRepo) FindBySlug(ctx context.Context, tenantID *uuid.UUID, slug string) (*domain.Role, error) {
	var role domain.Role
	q := r.db.WithContext(ctx).Where("slug = ? AND is_active = true", slug)
	if tenantID != nil {
		q = q.Where("tenant_id = ? OR tenant_id IS NULL", tenantID)
	} else {
		q = q.Where("tenant_id IS NULL")
	}
	if err := q.First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, apperrors.Wrap(err, "DB_ERROR", "find role by slug", 500)
	}
	return &role, nil
}

func (r *roleRepo) Create(ctx context.Context, role *domain.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

func (r *roleRepo) Update(ctx context.Context, role *domain.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

func (r *roleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&domain.Role{}).
		Where("id = ? AND is_system = false", id).
		Update("is_active", false).Error
}

// ──────────────────────────────────────────────
// Invitation Repository
// ──────────────────────────────────────────────

type invitationRepo struct{ db *gorm.DB }

// NewInvitationRepository creates a new InvitationRepository backed by PostgreSQL.
func NewInvitationRepository(db *gorm.DB) domain.InvitationRepository {
	return &invitationRepo{db: db}
}

func (r *invitationRepo) Create(ctx context.Context, inv *domain.Invitation) error {
	return r.db.WithContext(ctx).Create(inv).Error
}

func (r *invitationRepo) FindByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	var inv domain.Invitation
	if err := r.db.WithContext(ctx).
		Where("token = ? AND is_accepted = false AND expires_at > ?", token, time.Now()).
		First(&inv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrInvitationNotFound
		}
		return nil, apperrors.Wrap(err, "DB_ERROR", "find invitation by token", 500)
	}
	return &inv, nil
}

func (r *invitationRepo) FindByTenantAndEmail(ctx context.Context, tenantID uuid.UUID, email string) (*domain.Invitation, error) {
	var inv domain.Invitation
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND email = ? AND is_accepted = false AND expires_at > ?", tenantID, email, time.Now()).
		First(&inv).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrInvitationNotFound
		}
		return nil, apperrors.Wrap(err, "DB_ERROR", "find invitation by email", 500)
	}
	return &inv, nil
}

func (r *invitationRepo) Accept(ctx context.Context, token string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.Invitation{}).
		Where("token = ?", token).
		Updates(map[string]interface{}{"is_accepted": true, "accepted_at": now}).Error
}

func (r *invitationRepo) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ? AND is_accepted = false", time.Now()).
		Delete(&domain.Invitation{})
	return result.RowsAffected, result.Error
}
