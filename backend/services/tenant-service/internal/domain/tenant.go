package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ──────────────────────────────────────────────
// Entities
// ──────────────────────────────────────────────

// Tenant is a business entity (restaurant, cafe, retail store, etc.).
// Each tenant gets an isolated PostgreSQL schema.
type Tenant struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name         string         `gorm:"not null"`
	Slug         string         `gorm:"uniqueIndex;not null"` // URL-friendly identifier
	BusinessType BusinessType   `gorm:"not null"`
	OwnerID      uuid.UUID      `gorm:"type:uuid;not null;index"`
	PlanID       uuid.UUID      `gorm:"type:uuid"`
	Plan         string         `gorm:"not null;default:'starter'"`
	LogoURL      string
	Website      string
	Address      string
	City         string
	Province     string
	Country      string         `gorm:"default:'ID'"`
	Currency     string         `gorm:"default:'IDR'"`
	Timezone     string         `gorm:"default:'Asia/Jakarta'"`
	IsActive     bool           `gorm:"default:true;not null"`
	TrialEndsAt  *time.Time
	SchemaName   string         `gorm:"not null"` // e.g. tenant_550e8400...
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time `gorm:"index"`
}

// BusinessType defines the nature of the business.
type BusinessType string

const (
	BusinessTypeRetail     BusinessType = "retail"
	BusinessTypeFnB        BusinessType = "fnb"
	BusinessTypeService    BusinessType = "service"
	BusinessTypeCafe       BusinessType = "cafe"
	BusinessTypeRestaurant BusinessType = "restaurant"
	BusinessTypeGeneral    BusinessType = "general"
)

// Outlet is a physical location or branch of a Tenant.
// All POS transactions are scoped to an outlet.
type Outlet struct {
	ID            uuid.UUID    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID      uuid.UUID    `gorm:"type:uuid;not null;index"`
	Name          string       `gorm:"not null"`
	Code          string       `gorm:"not null"`    // Short code like "JKT-01"
	Phone         string
	Address       string
	City          string
	Province      string
	IsActive      bool         `gorm:"default:true;not null"`
	IsMainOutlet  bool         `gorm:"default:false"`
	OpenTime      string       `gorm:"default:'08:00'"`  // HH:MM
	CloseTime     string       `gorm:"default:'22:00'"`  // HH:MM
	TaxRate       float64      `gorm:"default:0.11"`     // Indonesia: 11% PPN
	TaxIncluded   bool         `gorm:"default:false"`
	ServiceCharge float64      `gorm:"default:0"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time   `gorm:"index"`
}

// TenantUser maps a user to a tenant with a specific role.
// A user can be in multiple tenants with different roles.
type TenantUser struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID   uuid.UUID `gorm:"type:uuid;not null;index"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	RoleID     uuid.UUID `gorm:"type:uuid;not null"`
	OutletID   *uuid.UUID `gorm:"type:uuid;index"` // nil = all outlets
	IsActive   bool      `gorm:"default:true;not null"`
	InvitedBy  uuid.UUID `gorm:"type:uuid"`
	JoinedAt   *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Role defines a set of permissions within a tenant.
type Role struct {
	ID          uuid.UUID    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    *uuid.UUID   `gorm:"type:uuid;index"` // nil = system role
	Name        string       `gorm:"not null"`
	Slug        string       `gorm:"not null"`
	Description string
	Permissions []string     `gorm:"type:text[];serializer:json"`
	IsSystem    bool         `gorm:"default:false;not null"` // System roles can't be deleted
	IsActive    bool         `gorm:"default:true;not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Invitation holds a pending invitation for a user to join a tenant.
type Invitation struct {
	ID          uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    uuid.UUID   `gorm:"type:uuid;not null;index"`
	Email       string      `gorm:"not null"`
	RoleID      uuid.UUID   `gorm:"type:uuid;not null"`
	OutletID    *uuid.UUID  `gorm:"type:uuid"`
	InvitedBy   uuid.UUID   `gorm:"type:uuid;not null"`
	Token       string      `gorm:"uniqueIndex;not null"`
	IsAccepted  bool        `gorm:"default:false;not null"`
	ExpiresAt   time.Time   `gorm:"not null"`
	AcceptedAt  *time.Time
	CreatedAt   time.Time
}

// ──────────────────────────────────────────────
// DTOs
// ──────────────────────────────────────────────

// CreateTenantInput holds data to create a new tenant (called by auth-service on register).
type CreateTenantInput struct {
	OwnerUserID  uuid.UUID    `json:"owner_user_id" validate:"required"`
	BusinessName string       `json:"business_name" validate:"required,min=2,max=200,xss"`
	BusinessType BusinessType `json:"business_type" validate:"required,oneof=retail fnb service cafe restaurant general"`
}

// UpdateTenantInput holds tenant profile update fields.
type UpdateTenantInput struct {
	Name     string `json:"name" validate:"omitempty,min=2,max=200,xss"`
	LogoURL  string `json:"logo_url" validate:"omitempty,url"`
	Website  string `json:"website" validate:"omitempty,url"`
	Address  string `json:"address" validate:"omitempty,max=500,xss"`
	City     string `json:"city" validate:"omitempty,max=100,xss"`
	Province string `json:"province" validate:"omitempty,max=100,xss"`
	Timezone string `json:"timezone" validate:"omitempty"`
}

// CreateOutletInput holds data to create a new outlet.
type CreateOutletInput struct {
	Name          string  `json:"name" validate:"required,min=2,max=200,xss"`
	Code          string  `json:"code" validate:"required,min=1,max=20"`
	Phone         string  `json:"phone" validate:"omitempty,phone_id"`
	Address       string  `json:"address" validate:"omitempty,max=500,xss"`
	City          string  `json:"city" validate:"omitempty,max=100,xss"`
	Province      string  `json:"province" validate:"omitempty,max=100,xss"`
	TaxRate       float64 `json:"tax_rate" validate:"gte=0,lte=1"`
	TaxIncluded   bool    `json:"tax_included"`
	ServiceCharge float64 `json:"service_charge" validate:"gte=0,lte=1"`
	OpenTime      string  `json:"open_time" validate:"omitempty"`
	CloseTime     string  `json:"close_time" validate:"omitempty"`
}

// InviteUserInput holds data to invite a user to a tenant.
type InviteUserInput struct {
	Email    string    `json:"email" validate:"required,email"`
	RoleID   uuid.UUID `json:"role_id" validate:"required,uuid4"`
	OutletID *uuid.UUID `json:"outlet_id" validate:"omitempty,uuid4"`
}

// ──────────────────────────────────────────────
// Domain Errors
// ──────────────────────────────────────────────

var (
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrOutletNotFound      = errors.New("outlet not found")
	ErrRoleNotFound        = errors.New("role not found")
	ErrSlugAlreadyExists   = errors.New("business slug already taken")
	ErrOutletCodeExists    = errors.New("outlet code already exists in this tenant")
	ErrMaxOutletsReached   = errors.New("maximum outlets limit reached for your plan")
	ErrMaxUsersReached     = errors.New("maximum users limit reached for your plan")
	ErrNotTenantMember     = errors.New("user is not a member of this tenant")
	ErrInvitationNotFound  = errors.New("invitation not found or expired")
	ErrAlreadyMember       = errors.New("user is already a member of this tenant")
	ErrCannotDeleteOwner   = errors.New("cannot remove the tenant owner")
	ErrSystemRole          = errors.New("system roles cannot be modified or deleted")
)

// PlanLimits defines feature limits per subscription plan.
var PlanLimits = map[string]struct {
	MaxOutlets int
	MaxUsers   int
	MaxProducts int
}{
	"free":       {MaxOutlets: 1, MaxUsers: 2, MaxProducts: 50},
	"starter":    {MaxOutlets: 1, MaxUsers: 5, MaxProducts: 200},
	"pro":        {MaxOutlets: 3, MaxUsers: 15, MaxProducts: 1000},
	"business":   {MaxOutlets: 10, MaxUsers: 50, MaxProducts: 10000},
	"enterprise": {MaxOutlets: 999, MaxUsers: 999, MaxProducts: 999999},
}
