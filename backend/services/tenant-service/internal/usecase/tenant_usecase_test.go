package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/extendedsynaptic/xynpos/tenant-service/internal/domain"
	"github.com/extendedsynaptic/xynpos/tenant-service/internal/usecase"
)

// ──────────────────────────────────────────────
// Mocks
// ──────────────────────────────────────────────

type mockTenantRepo struct{ mock.Mock }
type mockOutletRepo struct{ mock.Mock }
type mockMemberRepo struct{ mock.Mock }
type mockRoleRepo struct{ mock.Mock }
type mockInviteRepo struct{ mock.Mock }
type mockEventPublisher struct{ mock.Mock }

func (m *mockTenantRepo) Create(ctx context.Context, t *domain.Tenant) error {
	return m.Called(ctx, t).Error(0)
}
func (m *mockTenantRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}
func (m *mockTenantRepo) FindByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]*domain.Tenant, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]*domain.Tenant), args.Error(1)
}
func (m *mockTenantRepo) FindBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}
func (m *mockTenantRepo) Update(ctx context.Context, t *domain.Tenant) error {
	return m.Called(ctx, t).Error(0)
}
func (m *mockTenantRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockTenantRepo) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *mockOutletRepo) Create(ctx context.Context, o *domain.Outlet) error {
	return m.Called(ctx, o).Error(0)
}
func (m *mockOutletRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Outlet, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Outlet), args.Error(1)
}
func (m *mockOutletRepo) FindByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.Outlet, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*domain.Outlet), args.Error(1)
}
func (m *mockOutletRepo) Update(ctx context.Context, o *domain.Outlet) error {
	return m.Called(ctx, o).Error(0)
}
func (m *mockOutletRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockOutletRepo) CountByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockMemberRepo) Create(ctx context.Context, tu *domain.TenantUser) error {
	return m.Called(ctx, tu).Error(0)
}
func (m *mockMemberRepo) FindByTenantAndUser(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantUser, error) {
	args := m.Called(ctx, tenantID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TenantUser), args.Error(1)
}
func (m *mockMemberRepo) FindByTenantID(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantUser, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*domain.TenantUser), args.Error(1)
}
func (m *mockMemberRepo) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.TenantUser, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.TenantUser), args.Error(1)
}
func (m *mockMemberRepo) Update(ctx context.Context, tu *domain.TenantUser) error {
	return m.Called(ctx, tu).Error(0)
}
func (m *mockMemberRepo) Remove(ctx context.Context, tenantID, userID uuid.UUID) error {
	return m.Called(ctx, tenantID, userID).Error(0)
}
func (m *mockMemberRepo) CountByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockRoleRepo) FindAll(ctx context.Context, tenantID *uuid.UUID) ([]*domain.Role, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*domain.Role), args.Error(1)
}
func (m *mockRoleRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}
func (m *mockRoleRepo) FindBySlug(ctx context.Context, tenantID *uuid.UUID, slug string) (*domain.Role, error) {
	args := m.Called(ctx, tenantID, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Role), args.Error(1)
}
func (m *mockRoleRepo) Create(ctx context.Context, role *domain.Role) error {
	return m.Called(ctx, role).Error(0)
}
func (m *mockRoleRepo) Update(ctx context.Context, role *domain.Role) error {
	return m.Called(ctx, role).Error(0)
}
func (m *mockRoleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockInviteRepo) Create(ctx context.Context, inv *domain.Invitation) error {
	return m.Called(ctx, inv).Error(0)
}
func (m *mockInviteRepo) FindByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Invitation), args.Error(1)
}
func (m *mockInviteRepo) FindByTenantAndEmail(ctx context.Context, tenantID uuid.UUID, email string) (*domain.Invitation, error) {
	args := m.Called(ctx, tenantID, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Invitation), args.Error(1)
}
func (m *mockInviteRepo) Accept(ctx context.Context, token string) error {
	return m.Called(ctx, token).Error(0)
}
func (m *mockInviteRepo) DeleteExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockEventPublisher) PublishTenantCreated(ctx context.Context, tenantID, ownerID uuid.UUID, name, plan string) error {
	return m.Called(ctx, tenantID, ownerID, name, plan).Error(0)
}
func (m *mockEventPublisher) PublishUserInvited(ctx context.Context, tenantID uuid.UUID, email, token, tenantName string) error {
	return m.Called(ctx, tenantID, email, token, tenantName).Error(0)
}

// ──────────────────────────────────────────────
// Test helper: build usecase with a no-op real db for schema provisioning
// We use a nil DB — tests that call CreateTenant must mock schema provisioning
// or use usecase.NewWithDB(nil, ...)
// ──────────────────────────────────────────────

func newUC(
	tr *mockTenantRepo,
	or *mockOutletRepo,
	mr *mockMemberRepo,
	rr *mockRoleRepo,
	ir *mockInviteRepo,
	ep *mockEventPublisher,
) domain.TenantUsecase {
	// Pass nil DB — tests that exercise schema provisioning need integration setup
	// Unit tests for CreateTenant mock around it or use "no-op" db
	return usecase.New((*gorm.DB)(nil), tr, or, mr, rr, ir, ep)
}

// ──────────────────────────────────────────────
// Tests: UpdateTenant
// ──────────────────────────────────────────────

func TestUpdateTenant_Success(t *testing.T) {
	tenantID := uuid.New()
	ownerID := uuid.New()

	tr := &mockTenantRepo{}
	existing := &domain.Tenant{
		ID:      tenantID,
		OwnerID: ownerID,
		Name:    "Old Name",
		Plan:    "starter",
	}
	tr.On("FindByID", mock.Anything, tenantID).Return(existing, nil)
	tr.On("Update", mock.Anything, mock.MatchedBy(func(t *domain.Tenant) bool {
		return t.Name == "New Name"
	})).Return(nil)

	uc := newUC(tr, &mockOutletRepo{}, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})

	result, err := uc.UpdateTenant(context.Background(), tenantID, domain.UpdateTenantInput{
		Name: "New Name",
	})
	require.NoError(t, err)
	assert.Equal(t, "New Name", result.Name)
}

func TestUpdateTenant_TenantNotFound(t *testing.T) {
	tenantID := uuid.New()
	tr := &mockTenantRepo{}
	tr.On("FindByID", mock.Anything, tenantID).Return(nil, domain.ErrTenantNotFound)

	uc := newUC(tr, &mockOutletRepo{}, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})

	_, err := uc.UpdateTenant(context.Background(), tenantID, domain.UpdateTenantInput{Name: "X"})
	assert.ErrorIs(t, err, domain.ErrTenantNotFound)
}

// ──────────────────────────────────────────────
// Tests: CreateOutlet (plan limits)
// ──────────────────────────────────────────────

func TestCreateOutlet_Success(t *testing.T) {
	tenantID := uuid.New()
	tr := &mockTenantRepo{}
	or := &mockOutletRepo{}

	tr.On("FindByID", mock.Anything, tenantID).Return(&domain.Tenant{
		ID: tenantID, Plan: "pro", // Pro: 3 outlets allowed
	}, nil)
	or.On("CountByTenantID", mock.Anything, tenantID).Return(int64(1), nil) // 1 existing
	or.On("Create", mock.Anything, mock.MatchedBy(func(o *domain.Outlet) bool {
		return o.Name == "Branch 1" && o.Code == "BR1"
	})).Return(nil)

	uc := newUC(tr, or, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	outlet, err := uc.CreateOutlet(context.Background(), tenantID, domain.CreateOutletInput{
		Name: "Branch 1", Code: "BR1", TaxRate: 0.11,
	})

	require.NoError(t, err)
	assert.Equal(t, "Branch 1", outlet.Name)
	assert.Equal(t, tenantID, outlet.TenantID)
}

func TestCreateOutlet_StarterPlan_MaxReached(t *testing.T) {
	tenantID := uuid.New()
	tr := &mockTenantRepo{}
	or := &mockOutletRepo{}

	tr.On("FindByID", mock.Anything, tenantID).Return(&domain.Tenant{
		ID: tenantID, Plan: "starter", // Starter: max 1 outlet
	}, nil)
	or.On("CountByTenantID", mock.Anything, tenantID).Return(int64(1), nil) // Already at limit

	uc := newUC(tr, or, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	_, err := uc.CreateOutlet(context.Background(), tenantID, domain.CreateOutletInput{
		Name: "Second Outlet", Code: "BR2",
	})

	assert.ErrorIs(t, err, domain.ErrMaxOutletsReached)
}

func TestCreateOutlet_FreePlan_MaxReached(t *testing.T) {
	tenantID := uuid.New()
	tr := &mockTenantRepo{}
	or := &mockOutletRepo{}

	tr.On("FindByID", mock.Anything, tenantID).Return(&domain.Tenant{
		ID: tenantID, Plan: "free", // Free: max 1 outlet
	}, nil)
	or.On("CountByTenantID", mock.Anything, tenantID).Return(int64(1), nil)

	uc := newUC(tr, or, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	_, err := uc.CreateOutlet(context.Background(), tenantID, domain.CreateOutletInput{Name: "X", Code: "X"})
	assert.ErrorIs(t, err, domain.ErrMaxOutletsReached)
}

// ──────────────────────────────────────────────
// Tests: DeleteOutlet
// ──────────────────────────────────────────────

func TestDeleteOutlet_Success(t *testing.T) {
	tenantID := uuid.New()
	outletID := uuid.New()
	or := &mockOutletRepo{}

	or.On("FindByID", mock.Anything, outletID).Return(&domain.Outlet{
		ID: outletID, TenantID: tenantID, IsMainOutlet: false,
	}, nil)
	or.On("SoftDelete", mock.Anything, outletID).Return(nil)

	uc := newUC(&mockTenantRepo{}, or, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	err := uc.DeleteOutlet(context.Background(), tenantID, outletID)
	assert.NoError(t, err)
}

func TestDeleteOutlet_CannotDeleteMainOutlet(t *testing.T) {
	tenantID := uuid.New()
	outletID := uuid.New()
	or := &mockOutletRepo{}

	or.On("FindByID", mock.Anything, outletID).Return(&domain.Outlet{
		ID: outletID, TenantID: tenantID, IsMainOutlet: true, // Main outlet
	}, nil)

	uc := newUC(&mockTenantRepo{}, or, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	err := uc.DeleteOutlet(context.Background(), tenantID, outletID)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, domain.ErrOutletNotFound)) // Different error
}

func TestDeleteOutlet_WrongTenant(t *testing.T) {
	tenantID := uuid.New()
	otherTenantID := uuid.New()
	outletID := uuid.New()
	or := &mockOutletRepo{}

	or.On("FindByID", mock.Anything, outletID).Return(&domain.Outlet{
		ID: outletID, TenantID: otherTenantID, IsMainOutlet: false,
	}, nil)

	uc := newUC(&mockTenantRepo{}, or, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	err := uc.DeleteOutlet(context.Background(), tenantID, outletID)
	// Must be forbidden — can't delete another tenant's outlet
	assert.Error(t, err)
}

// ──────────────────────────────────────────────
// Tests: GetOutlets
// ──────────────────────────────────────────────

func TestGetOutlets_Success(t *testing.T) {
	tenantID := uuid.New()
	or := &mockOutletRepo{}

	or.On("FindByTenantID", mock.Anything, tenantID).Return([]*domain.Outlet{
		{ID: uuid.New(), TenantID: tenantID, Name: "Main", IsMainOutlet: true},
		{ID: uuid.New(), TenantID: tenantID, Name: "Branch A"},
	}, nil)

	uc := newUC(&mockTenantRepo{}, or, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	outlets, err := uc.GetOutlets(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Len(t, outlets, 2)
}

// ──────────────────────────────────────────────
// Tests: GetMyTenants
// ──────────────────────────────────────────────

func TestGetMyTenants_MultiTenant(t *testing.T) {
	userID := uuid.New()
	tenant1ID := uuid.New()
	tenant2ID := uuid.New()

	mr := &mockMemberRepo{}
	tr := &mockTenantRepo{}

	mr.On("FindByUserID", mock.Anything, userID).Return([]*domain.TenantUser{
		{TenantID: tenant1ID, UserID: userID, IsActive: true},
		{TenantID: tenant2ID, UserID: userID, IsActive: true},
	}, nil)
	tr.On("FindByID", mock.Anything, tenant1ID).Return(&domain.Tenant{ID: tenant1ID, Name: "Business A"}, nil)
	tr.On("FindByID", mock.Anything, tenant2ID).Return(&domain.Tenant{ID: tenant2ID, Name: "Business B"}, nil)

	uc := newUC(tr, &mockOutletRepo{}, mr, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	tenants, err := uc.GetMyTenants(context.Background(), userID)
	require.NoError(t, err)
	assert.Len(t, tenants, 2)
}

// ──────────────────────────────────────────────
// Tests: Member management
// ──────────────────────────────────────────────

func TestRemoveMember_CannotRemoveOwner(t *testing.T) {
	ownerID := uuid.New()
	tenantID := uuid.New()
	tr := &mockTenantRepo{}

	tr.On("FindByID", mock.Anything, tenantID).Return(&domain.Tenant{
		ID: tenantID, OwnerID: ownerID,
	}, nil)

	uc := newUC(tr, &mockOutletRepo{}, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	err := uc.RemoveMember(context.Background(), tenantID, ownerID, ownerID)
	assert.ErrorIs(t, err, domain.ErrCannotDeleteOwner)
}

func TestRemoveMember_Success(t *testing.T) {
	ownerID := uuid.New()
	staffID := uuid.New()
	tenantID := uuid.New()
	tr := &mockTenantRepo{}
	mr := &mockMemberRepo{}

	tr.On("FindByID", mock.Anything, tenantID).Return(&domain.Tenant{
		ID: tenantID, OwnerID: ownerID,
	}, nil)
	mr.On("Remove", mock.Anything, tenantID, staffID).Return(nil)

	uc := newUC(tr, &mockOutletRepo{}, mr, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	err := uc.RemoveMember(context.Background(), tenantID, ownerID, staffID)
	assert.NoError(t, err)
}

// ──────────────────────────────────────────────
// Tests: AcceptInvitation
// ──────────────────────────────────────────────

func TestAcceptInvitation_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	roleID := uuid.New()
	token := "valid-invite-token"

	ir := &mockInviteRepo{}
	mr := &mockMemberRepo{}

	ir.On("FindByToken", mock.Anything, token).Return(&domain.Invitation{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "staff@example.com",
		RoleID:   roleID,
		Token:    token,
	}, nil)
	mr.On("Create", mock.Anything, mock.MatchedBy(func(tu *domain.TenantUser) bool {
		return tu.UserID == userID && tu.TenantID == tenantID
	})).Return(nil)
	ir.On("Accept", mock.Anything, token).Return(nil)

	uc := newUC(&mockTenantRepo{}, &mockOutletRepo{}, mr, &mockRoleRepo{}, ir, &mockEventPublisher{})
	err := uc.AcceptInvitation(context.Background(), userID, token)
	assert.NoError(t, err)
}

func TestAcceptInvitation_InvalidToken(t *testing.T) {
	ir := &mockInviteRepo{}
	ir.On("FindByToken", mock.Anything, "expired-token").Return(nil, domain.ErrInvitationNotFound)

	uc := newUC(&mockTenantRepo{}, &mockOutletRepo{}, &mockMemberRepo{}, &mockRoleRepo{}, ir, &mockEventPublisher{})
	err := uc.AcceptInvitation(context.Background(), uuid.New(), "expired-token")
	assert.ErrorIs(t, err, domain.ErrInvitationNotFound)
}

// ──────────────────────────────────────────────
// Tests: GetUserRole
// ──────────────────────────────────────────────

func TestGetUserRole_Success(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	roleID := uuid.New()

	mr := &mockMemberRepo{}
	rr := &mockRoleRepo{}

	mr.On("FindByTenantAndUser", mock.Anything, tenantID, userID).Return(&domain.TenantUser{
		TenantID: tenantID,
		UserID:   userID,
		RoleID:   roleID,
		IsActive: true,
	}, nil)
	rr.On("FindByID", mock.Anything, roleID).Return(&domain.Role{
		ID:          roleID,
		Name:        "Cashier",
		Slug:        "cashier",
		Permissions: []string{"product:read", "transaction:*"},
	}, nil)

	uc := newUC(&mockTenantRepo{}, &mockOutletRepo{}, mr, rr, &mockInviteRepo{}, &mockEventPublisher{})
	role, permissions, err := uc.GetUserRole(context.Background(), tenantID, userID)
	require.NoError(t, err)
	assert.Equal(t, "cashier", role.Slug)
	assert.Contains(t, permissions, "transaction:*")
}

func TestGetUserRole_UserNotMember(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	mr := &mockMemberRepo{}
	mr.On("FindByTenantAndUser", mock.Anything, tenantID, userID).Return(nil, domain.ErrNotTenantMember)

	uc := newUC(&mockTenantRepo{}, &mockOutletRepo{}, mr, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	_, _, err := uc.GetUserRole(context.Background(), tenantID, userID)
	assert.ErrorIs(t, err, domain.ErrNotTenantMember)
}

// ──────────────────────────────────────────────
// Tests: slug generation (tested via domain helper)
// ──────────────────────────────────────────────
// Note: CreateTenant calls provisionTenantSchema which requires a real DB.
// Slug generation is an internal pure function — test the observable behavior
// by calling UpdateTenant and verifying slugs via GetTenant lookup.

func TestGetTenant_NotFound(t *testing.T) {
	tenantID := uuid.New()
	tr := &mockTenantRepo{}
	tr.On("FindByID", mock.Anything, tenantID).Return(nil, domain.ErrTenantNotFound)

	uc := newUC(tr, &mockOutletRepo{}, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	_, err := uc.GetTenant(context.Background(), tenantID)
	assert.ErrorIs(t, err, domain.ErrTenantNotFound)
}

func TestGetTenant_Success(t *testing.T) {
	tenantID := uuid.New()
	tr := &mockTenantRepo{}
	tr.On("FindByID", mock.Anything, tenantID).Return(&domain.Tenant{
		ID:   tenantID,
		Name: "My Store",
		Plan: "pro",
	}, nil)

	uc := newUC(tr, &mockOutletRepo{}, &mockMemberRepo{}, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	tenant, err := uc.GetTenant(context.Background(), tenantID)
	require.NoError(t, err)
	assert.Equal(t, "My Store", tenant.Name)
}

// ──────────────────────────────────────────────
// Tests: InviteUser — plan limits
// ──────────────────────────────────────────────

func TestInviteUser_MaxUsersReached(t *testing.T) {
	tenantID := uuid.New()
	inviterID := uuid.New()

	tr := &mockTenantRepo{}
	mr := &mockMemberRepo{}

	tr.On("FindByID", mock.Anything, tenantID).Return(&domain.Tenant{
		ID: tenantID, Name: "My Business", Plan: "starter", // max 5 users
	}, nil)
	mr.On("CountByTenantID", mock.Anything, tenantID).Return(int64(5), nil) // Already at limit

	uc := newUC(tr, &mockOutletRepo{}, mr, &mockRoleRepo{}, &mockInviteRepo{}, &mockEventPublisher{})
	err := uc.InviteUser(context.Background(), tenantID, inviterID, domain.InviteUserInput{
		Email:  "newstaff@example.com",
		RoleID: uuid.New(),
	})
	assert.ErrorIs(t, err, domain.ErrMaxUsersReached)
}
