package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/extendedsynaptic/xynpos/tenant-service/internal/domain"
	"github.com/extendedsynaptic/xynpos/tenant-service/internal/event"
	appdb "github.com/extendedsynaptic/xynpos/shared/pkg/database"
	apperrors "github.com/extendedsynaptic/xynpos/shared/pkg/errors"
	"github.com/extendedsynaptic/xynpos/shared/pkg/logger"
	"github.com/extendedsynaptic/xynpos/shared/pkg/tracer"
	"gorm.io/gorm"
)

// tenantUsecase implements domain.TenantUsecase.
type tenantUsecase struct {
	db          *gorm.DB // for schema provisioning
	tenantRepo  domain.TenantRepository
	outletRepo  domain.OutletRepository
	memberRepo  domain.TenantUserRepository
	roleRepo    domain.RoleRepository
	inviteRepo  domain.InvitationRepository
	events      event.Publisher
}

// New creates a new TenantUsecase.
func New(
	db *gorm.DB,
	tenantRepo domain.TenantRepository,
	outletRepo domain.OutletRepository,
	memberRepo domain.TenantUserRepository,
	roleRepo domain.RoleRepository,
	inviteRepo domain.InvitationRepository,
	events event.Publisher,
) domain.TenantUsecase {
	return &tenantUsecase{
		db:         db,
		tenantRepo: tenantRepo,
		outletRepo: outletRepo,
		memberRepo: memberRepo,
		roleRepo:   roleRepo,
		inviteRepo: inviteRepo,
		events:     events,
	}
}

// ──────────────────────────────────────────────
// CreateTenant
// ──────────────────────────────────────────────

func (uc *tenantUsecase) CreateTenant(ctx context.Context, ownerID uuid.UUID, input domain.CreateTenantInput) (*domain.Tenant, error) {
	ctx, span := tracer.StartSpan(ctx, "TenantUsecase.CreateTenant")
	defer span.End()

	log := logger.FromContext(ctx)

	// Generate URL-safe slug from business name
	slug := generateSlug(input.BusinessName)

	// Ensure slug uniqueness
	finalSlug, err := uc.ensureUniqueSlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	tenantID := uuid.New()
	schemaName := appdb.TenantSchemaName(tenantID.String())

	tenant := &domain.Tenant{
		ID:           tenantID,
		Name:         input.BusinessName,
		Slug:         finalSlug,
		BusinessType: input.BusinessType,
		OwnerID:      ownerID,
		Plan:         "starter",
		Country:      "ID",
		Currency:     "IDR",
		Timezone:     "Asia/Jakarta",
		IsActive:     true,
		SchemaName:   schemaName,
	}

	if err := uc.tenantRepo.Create(ctx, tenant); err != nil {
		tracer.RecordError(span, err)
		return nil, err
	}

	// Provision PostgreSQL schema for this tenant
	if err := uc.provisionTenantSchema(ctx, schemaName); err != nil {
		tracer.RecordError(span, err)
		log.Error("failed to provision tenant schema, rolling back", zap.Error(err), zap.String("tenant_id", tenantID.String()))
		_ = uc.tenantRepo.SoftDelete(ctx, tenantID)
		return nil, apperrors.Wrap(err, "SCHEMA_PROVISION_ERROR", "failed to create tenant database schema", 500)
	}

	// Add owner as tenant member with 'owner' role
	ownerRole, err := uc.roleRepo.FindBySlug(ctx, nil, "owner")
	if err != nil {
		log.Warn("owner role not found, using default", zap.Error(err))
	}

	now := time.Now()
	tu := &domain.TenantUser{
		ID:       uuid.New(),
		TenantID: tenantID,
		UserID:   ownerID,
		IsActive: true,
		JoinedAt: &now,
	}
	if ownerRole != nil {
		tu.RoleID = ownerRole.ID
	}
	if err := uc.memberRepo.Create(ctx, tu); err != nil {
		log.Warn("failed to add owner as tenant member", zap.Error(err))
	}

	// Create default main outlet
	mainOutlet := &domain.Outlet{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Name:         input.BusinessName,
		Code:         "MAIN",
		IsActive:     true,
		IsMainOutlet: true,
		OpenTime:     "08:00",
		CloseTime:    "22:00",
		TaxRate:      0.11, // 11% PPN Indonesia
		TaxIncluded:  false,
	}
	if err := uc.outletRepo.Create(ctx, mainOutlet); err != nil {
		log.Warn("failed to create main outlet", zap.Error(err))
	}

	// Publish event (async — non-blocking)
	_ = uc.events.PublishTenantCreated(ctx, tenantID, ownerID, tenant.Name, tenant.Plan)

	log.Info("tenant created",
		zap.String("tenant_id", tenantID.String()),
		zap.String("slug", finalSlug),
		zap.String("schema", schemaName),
	)

	return tenant, nil
}

// ──────────────────────────────────────────────
// GetTenant
// ──────────────────────────────────────────────

func (uc *tenantUsecase) GetTenant(ctx context.Context, tenantID uuid.UUID) (*domain.Tenant, error) {
	return uc.tenantRepo.FindByID(ctx, tenantID)
}

// ──────────────────────────────────────────────
// UpdateTenant
// ──────────────────────────────────────────────

func (uc *tenantUsecase) UpdateTenant(ctx context.Context, tenantID uuid.UUID, input domain.UpdateTenantInput) (*domain.Tenant, error) {
	ctx, span := tracer.StartSpan(ctx, "TenantUsecase.UpdateTenant")
	defer span.End()

	tenant, err := uc.tenantRepo.FindByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		tenant.Name = input.Name
	}
	if input.LogoURL != "" {
		tenant.LogoURL = input.LogoURL
	}
	if input.Website != "" {
		tenant.Website = input.Website
	}
	if input.Address != "" {
		tenant.Address = input.Address
	}
	if input.City != "" {
		tenant.City = input.City
	}
	if input.Province != "" {
		tenant.Province = input.Province
	}
	if input.Timezone != "" {
		tenant.Timezone = input.Timezone
	}

	if err := uc.tenantRepo.Update(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

// ──────────────────────────────────────────────
// GetMyTenants
// ──────────────────────────────────────────────

func (uc *tenantUsecase) GetMyTenants(ctx context.Context, userID uuid.UUID) ([]*domain.Tenant, error) {
	// Get all tenant memberships for this user
	memberships, err := uc.memberRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var tenants []*domain.Tenant
	for _, m := range memberships {
		t, err := uc.tenantRepo.FindByID(ctx, m.TenantID)
		if err != nil {
			continue
		}
		tenants = append(tenants, t)
	}
	return tenants, nil
}

// ──────────────────────────────────────────────
// Outlet methods
// ──────────────────────────────────────────────

func (uc *tenantUsecase) CreateOutlet(ctx context.Context, tenantID uuid.UUID, input domain.CreateOutletInput) (*domain.Outlet, error) {
	ctx, span := tracer.StartSpan(ctx, "TenantUsecase.CreateOutlet")
	defer span.End()

	// Get tenant to check plan limits
	tenant, err := uc.tenantRepo.FindByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Check plan limits
	limits, ok := domain.PlanLimits[tenant.Plan]
	if ok {
		count, err := uc.outletRepo.CountByTenantID(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		if int(count) >= limits.MaxOutlets {
			return nil, domain.ErrMaxOutletsReached
		}
	}

	outlet := &domain.Outlet{
		ID:            uuid.New(),
		TenantID:      tenantID,
		Name:          input.Name,
		Code:          input.Code,
		Phone:         input.Phone,
		Address:       input.Address,
		City:          input.City,
		Province:      input.Province,
		IsActive:      true,
		IsMainOutlet:  false,
		TaxRate:       input.TaxRate,
		TaxIncluded:   input.TaxIncluded,
		ServiceCharge: input.ServiceCharge,
		OpenTime:      input.OpenTime,
		CloseTime:     input.CloseTime,
	}
	if outlet.OpenTime == "" {
		outlet.OpenTime = "08:00"
	}
	if outlet.CloseTime == "" {
		outlet.CloseTime = "22:00"
	}

	if err := uc.outletRepo.Create(ctx, outlet); err != nil {
		return nil, err
	}
	return outlet, nil
}

func (uc *tenantUsecase) GetOutlets(ctx context.Context, tenantID uuid.UUID) ([]*domain.Outlet, error) {
	return uc.outletRepo.FindByTenantID(ctx, tenantID)
}

func (uc *tenantUsecase) UpdateOutlet(ctx context.Context, tenantID, outletID uuid.UUID, input domain.CreateOutletInput) (*domain.Outlet, error) {
	outlet, err := uc.outletRepo.FindByID(ctx, outletID)
	if err != nil {
		return nil, err
	}
	if outlet.TenantID != tenantID {
		return nil, apperrors.ErrForbidden
	}

	outlet.Name = input.Name
	outlet.Code = input.Code
	outlet.Phone = input.Phone
	outlet.Address = input.Address
	outlet.City = input.City
	outlet.Province = input.Province
	outlet.TaxRate = input.TaxRate
	outlet.TaxIncluded = input.TaxIncluded
	outlet.ServiceCharge = input.ServiceCharge
	if input.OpenTime != "" {
		outlet.OpenTime = input.OpenTime
	}
	if input.CloseTime != "" {
		outlet.CloseTime = input.CloseTime
	}

	if err := uc.outletRepo.Update(ctx, outlet); err != nil {
		return nil, err
	}
	return outlet, nil
}

func (uc *tenantUsecase) DeleteOutlet(ctx context.Context, tenantID, outletID uuid.UUID) error {
	outlet, err := uc.outletRepo.FindByID(ctx, outletID)
	if err != nil {
		return err
	}
	if outlet.TenantID != tenantID {
		return apperrors.ErrForbidden
	}
	if outlet.IsMainOutlet {
		return apperrors.New("CANNOT_DELETE_MAIN_OUTLET", "Cannot delete the main outlet", 400)
	}
	return uc.outletRepo.SoftDelete(ctx, outletID)
}

// ──────────────────────────────────────────────
// Member management
// ──────────────────────────────────────────────

func (uc *tenantUsecase) InviteUser(ctx context.Context, tenantID uuid.UUID, invitedBy uuid.UUID, input domain.InviteUserInput) error {
	ctx, span := tracer.StartSpan(ctx, "TenantUsecase.InviteUser")
	defer span.End()

	// Check plan limits
	tenant, err := uc.tenantRepo.FindByID(ctx, tenantID)
	if err != nil {
		return err
	}
	limits, ok := domain.PlanLimits[tenant.Plan]
	if ok {
		count, _ := uc.memberRepo.CountByTenantID(ctx, tenantID)
		if int(count) >= limits.MaxUsers {
			return domain.ErrMaxUsersReached
		}
	}

	// Generate secure invitation token
	token, err := generateToken(32)
	if err != nil {
		return fmt.Errorf("invite user: generate token: %w", err)
	}

	inv := &domain.Invitation{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Email:     input.Email,
		RoleID:    input.RoleID,
		OutletID:  input.OutletID,
		InvitedBy: invitedBy,
		Token:     token,
		ExpiresAt: time.Now().Add(72 * time.Hour), // 3 days
	}
	if err := uc.inviteRepo.Create(ctx, inv); err != nil {
		return err
	}

	// Publish event → notification-service sends invitation email
	return uc.events.PublishUserInvited(ctx, tenantID, input.Email, token, tenant.Name)
}

func (uc *tenantUsecase) AcceptInvitation(ctx context.Context, userID uuid.UUID, token string) error {
	ctx, span := tracer.StartSpan(ctx, "TenantUsecase.AcceptInvitation")
	defer span.End()

	inv, err := uc.inviteRepo.FindByToken(ctx, token)
	if err != nil {
		return err
	}

	// Add user as tenant member
	now := time.Now()
	tu := &domain.TenantUser{
		ID:        uuid.New(),
		TenantID:  inv.TenantID,
		UserID:    userID,
		RoleID:    inv.RoleID,
		OutletID:  inv.OutletID,
		IsActive:  true,
		InvitedBy: inv.InvitedBy,
		JoinedAt:  &now,
	}
	if err := uc.memberRepo.Create(ctx, tu); err != nil {
		return err
	}

	return uc.inviteRepo.Accept(ctx, token)
}

func (uc *tenantUsecase) GetMembers(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantUser, error) {
	return uc.memberRepo.FindByTenantID(ctx, tenantID)
}

func (uc *tenantUsecase) RemoveMember(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID) error {
	// Check if target is the owner
	tenant, err := uc.tenantRepo.FindByID(ctx, tenantID)
	if err != nil {
		return err
	}
	if tenant.OwnerID == targetUserID {
		return domain.ErrCannotDeleteOwner
	}
	return uc.memberRepo.Remove(ctx, tenantID, targetUserID)
}

func (uc *tenantUsecase) UpdateMemberRole(ctx context.Context, tenantID, userID, newRoleID uuid.UUID) error {
	tu, err := uc.memberRepo.FindByTenantAndUser(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	tu.RoleID = newRoleID
	return uc.memberRepo.Update(ctx, tu)
}

// ──────────────────────────────────────────────
// Roles
// ──────────────────────────────────────────────

func (uc *tenantUsecase) GetRoles(ctx context.Context, tenantID uuid.UUID) ([]*domain.Role, error) {
	return uc.roleRepo.FindAll(ctx, &tenantID)
}

func (uc *tenantUsecase) GetUserRole(ctx context.Context, tenantID, userID uuid.UUID) (*domain.Role, []string, error) {
	tu, err := uc.memberRepo.FindByTenantAndUser(ctx, tenantID, userID)
	if err != nil {
		return nil, nil, err
	}

	role, err := uc.roleRepo.FindByID(ctx, tu.RoleID)
	if err != nil {
		return nil, nil, err
	}

	return role, role.Permissions, nil
}

// ──────────────────────────────────────────────
// Private helpers
// ──────────────────────────────────────────────

var (
	reNonAlphanumeric = regexp.MustCompile(`[^a-z0-9\s-]`)
	reSpaces          = regexp.MustCompile(`[\s-]+`)
)

// generateSlug creates a URL-safe slug from a business name.
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = reNonAlphanumeric.ReplaceAllString(slug, "")
	slug = reSpaces.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 60 {
		slug = slug[:60]
	}
	return slug
}

// ensureUniqueSlug appends a suffix if the slug already exists.
func (uc *tenantUsecase) ensureUniqueSlug(ctx context.Context, base string) (string, error) {
	slug := base
	for i := 1; i <= 10; i++ {
		_, err := uc.tenantRepo.FindBySlug(ctx, slug)
		if err != nil {
			// Not found = slug is available
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return "", apperrors.New("SLUG_CONFLICT", "Could not generate a unique slug after 10 attempts", 500)
}

// provisionTenantSchema creates the PostgreSQL schema for a new tenant.
// Also creates the outlet table in the tenant schema.
func (uc *tenantUsecase) provisionTenantSchema(ctx context.Context, schemaName string) error {
	if err := appdb.ValidateSchemaName(schemaName); err != nil {
		return err
	}

	sqls := []string{
		// Create schema
		fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName),
		// Outlets table in tenant schema
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.outlets (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id       UUID NOT NULL,
			name            VARCHAR(200) NOT NULL,
			code            VARCHAR(20) NOT NULL,
			phone           VARCHAR(20),
			address         TEXT,
			city            VARCHAR(100),
			province        VARCHAR(100),
			is_active       BOOLEAN NOT NULL DEFAULT true,
			is_main_outlet  BOOLEAN NOT NULL DEFAULT false,
			open_time       VARCHAR(5) NOT NULL DEFAULT '08:00',
			close_time      VARCHAR(5) NOT NULL DEFAULT '22:00',
			tax_rate        DECIMAL(5,4) NOT NULL DEFAULT 0.11,
			tax_included    BOOLEAN NOT NULL DEFAULT false,
			service_charge  DECIMAL(5,4) NOT NULL DEFAULT 0,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at      TIMESTAMPTZ,
			CONSTRAINT %s_outlets_code_unique UNIQUE (code)
		)`, schemaName, schemaName),
		// Grant usage on schema to app user
		fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO xynpos", schemaName),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA %s TO xynpos", schemaName),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA %s GRANT ALL ON TABLES TO xynpos", schemaName),
	}

	for _, sql := range sqls {
		if err := uc.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("provision schema %s: %w", schemaName, err)
		}
	}

	return nil
}

// generateToken creates a cryptographically secure random hex token.
func generateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
