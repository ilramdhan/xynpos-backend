package domain

import (
	"context"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Repository Interfaces
// ──────────────────────────────────────────────

// TenantRepository defines persistence for Tenant.
type TenantRepository interface {
	Create(ctx context.Context, tenant *Tenant) error
	FindByID(ctx context.Context, id uuid.UUID) (*Tenant, error)
	FindByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]*Tenant, error)
	FindBySlug(ctx context.Context, slug string) (*Tenant, error)
	Update(ctx context.Context, tenant *Tenant) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// OutletRepository defines persistence for Outlet (lives in tenant schema).
type OutletRepository interface {
	Create(ctx context.Context, outlet *Outlet) error
	FindByID(ctx context.Context, id uuid.UUID) (*Outlet, error)
	FindByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*Outlet, error)
	Update(ctx context.Context, outlet *Outlet) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	CountByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error)
}

// TenantUserRepository defines persistence for TenantUser.
type TenantUserRepository interface {
	Create(ctx context.Context, tu *TenantUser) error
	FindByTenantAndUser(ctx context.Context, tenantID, userID uuid.UUID) (*TenantUser, error)
	FindByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*TenantUser, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*TenantUser, error)
	Update(ctx context.Context, tu *TenantUser) error
	Remove(ctx context.Context, tenantID, userID uuid.UUID) error
	CountByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error)
}

// RoleRepository defines persistence for Role.
type RoleRepository interface {
	FindAll(ctx context.Context, tenantID *uuid.UUID) ([]*Role, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Role, error)
	FindBySlug(ctx context.Context, tenantID *uuid.UUID, slug string) (*Role, error)
	Create(ctx context.Context, role *Role) error
	Update(ctx context.Context, role *Role) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// InvitationRepository defines persistence for Invitation.
type InvitationRepository interface {
	Create(ctx context.Context, inv *Invitation) error
	FindByToken(ctx context.Context, token string) (*Invitation, error)
	FindByTenantAndEmail(ctx context.Context, tenantID uuid.UUID, email string) (*Invitation, error)
	Accept(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// ──────────────────────────────────────────────
// Usecase Interface
// ──────────────────────────────────────────────

// TenantUsecase defines all business operations for the tenant service.
type TenantUsecase interface {
	// CreateTenant creates a new tenant schema and sets up default roles + main outlet.
	// Called by auth-service on user registration.
	CreateTenant(ctx context.Context, ownerID uuid.UUID, input CreateTenantInput) (*Tenant, error)

	// GetTenant retrieves a tenant by ID (requester must be a member).
	GetTenant(ctx context.Context, tenantID uuid.UUID) (*Tenant, error)

	// UpdateTenant updates tenant profile.
	UpdateTenant(ctx context.Context, tenantID uuid.UUID, input UpdateTenantInput) (*Tenant, error)

	// GetMyTenants returns all tenants the user belongs to.
	GetMyTenants(ctx context.Context, userID uuid.UUID) ([]*Tenant, error)

	// ── Outlets ──────────────────────────────────────────────

	// CreateOutlet creates a new outlet (checks plan limits).
	CreateOutlet(ctx context.Context, tenantID uuid.UUID, input CreateOutletInput) (*Outlet, error)

	// GetOutlets returns all outlets for a tenant.
	GetOutlets(ctx context.Context, tenantID uuid.UUID) ([]*Outlet, error)

	// UpdateOutlet updates outlet details.
	UpdateOutlet(ctx context.Context, tenantID, outletID uuid.UUID, input CreateOutletInput) (*Outlet, error)

	// DeleteOutlet soft-deletes an outlet.
	DeleteOutlet(ctx context.Context, tenantID, outletID uuid.UUID) error

	// ── Members ──────────────────────────────────────────────

	// InviteUser sends an invitation email to join the tenant.
	InviteUser(ctx context.Context, tenantID uuid.UUID, invitedBy uuid.UUID, input InviteUserInput) error

	// AcceptInvitation accepts a pending invitation.
	AcceptInvitation(ctx context.Context, userID uuid.UUID, token string) error

	// GetMembers returns all active members of a tenant.
	GetMembers(ctx context.Context, tenantID uuid.UUID) ([]*TenantUser, error)

	// RemoveMember removes a member from the tenant.
	RemoveMember(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID) error

	// UpdateMemberRole changes a member's role.
	UpdateMemberRole(ctx context.Context, tenantID, userID, newRoleID uuid.UUID) error

	// ── Roles ────────────────────────────────────────────────

	// GetRoles returns all roles for a tenant (including system roles).
	GetRoles(ctx context.Context, tenantID uuid.UUID) ([]*Role, error)

	// GetUserRole returns the role for a user within a tenant (used by auth-service).
	GetUserRole(ctx context.Context, tenantID, userID uuid.UUID) (*Role, []string, error)
}
