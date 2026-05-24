package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/extendedsynaptic/xynpos/tenant-service/internal/delivery/http/handler"
	"github.com/extendedsynaptic/xynpos/tenant-service/internal/domain"
)

// ──────────────────────────────────────────────
// Mock TenantUsecase
// ──────────────────────────────────────────────

type mockTenantUsecase struct{ mock.Mock }

func (m *mockTenantUsecase) CreateTenant(ctx context.Context, ownerID uuid.UUID, input domain.CreateTenantInput) (*domain.Tenant, error) {
	args := m.Called(ctx, ownerID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}

func (m *mockTenantUsecase) GetTenant(ctx context.Context, tenantID uuid.UUID) (*domain.Tenant, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}

func (m *mockTenantUsecase) UpdateTenant(ctx context.Context, tenantID uuid.UUID, input domain.UpdateTenantInput) (*domain.Tenant, error) {
	args := m.Called(ctx, tenantID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tenant), args.Error(1)
}

func (m *mockTenantUsecase) GetMyTenants(ctx context.Context, userID uuid.UUID) ([]*domain.Tenant, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tenant), args.Error(1)
}

func (m *mockTenantUsecase) CreateOutlet(ctx context.Context, tenantID uuid.UUID, input domain.CreateOutletInput) (*domain.Outlet, error) {
	args := m.Called(ctx, tenantID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Outlet), args.Error(1)
}

func (m *mockTenantUsecase) GetOutlets(ctx context.Context, tenantID uuid.UUID) ([]*domain.Outlet, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Outlet), args.Error(1)
}

func (m *mockTenantUsecase) UpdateOutlet(ctx context.Context, tenantID, outletID uuid.UUID, input domain.CreateOutletInput) (*domain.Outlet, error) {
	args := m.Called(ctx, tenantID, outletID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Outlet), args.Error(1)
}

func (m *mockTenantUsecase) DeleteOutlet(ctx context.Context, tenantID, outletID uuid.UUID) error {
	return m.Called(ctx, tenantID, outletID).Error(0)
}

func (m *mockTenantUsecase) InviteUser(ctx context.Context, tenantID uuid.UUID, invitedBy uuid.UUID, input domain.InviteUserInput) error {
	return m.Called(ctx, tenantID, invitedBy, input).Error(0)
}

func (m *mockTenantUsecase) AcceptInvitation(ctx context.Context, userID uuid.UUID, token string) error {
	return m.Called(ctx, userID, token).Error(0)
}

func (m *mockTenantUsecase) GetMembers(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantUser, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TenantUser), args.Error(1)
}

func (m *mockTenantUsecase) RemoveMember(ctx context.Context, tenantID, requesterID, targetUserID uuid.UUID) error {
	return m.Called(ctx, tenantID, requesterID, targetUserID).Error(0)
}

func (m *mockTenantUsecase) UpdateMemberRole(ctx context.Context, tenantID, userID, newRoleID uuid.UUID) error {
	return m.Called(ctx, tenantID, userID, newRoleID).Error(0)
}

func (m *mockTenantUsecase) GetRoles(ctx context.Context, tenantID uuid.UUID) ([]*domain.Role, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Role), args.Error(1)
}

func (m *mockTenantUsecase) GetUserRole(ctx context.Context, tenantID, userID uuid.UUID) (*domain.Role, []string, error) {
	args := m.Called(ctx, tenantID, userID)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*domain.Role), args.Get(1).([]string), args.Error(2)
}

// ──────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────

func setupApp(uc domain.TenantUsecase, userID string) *fiber.App {
	app := fiber.New()
	h := handler.NewTenantHandler(uc)
	authMW := func(c fiber.Ctx) error {
		c.Locals("userID", userID)
		c.Locals("tenantID", uuid.New().String())
		return c.Next()
	}
	h.Register(app, authMW)
	return app
}

func doRequest(app *fiber.App, method, path string, body interface{}) (int, map[string]interface{}) {
	var b io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		b = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, b)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return resp.StatusCode, result
}

func fakeTenant(id uuid.UUID) *domain.Tenant {
	return &domain.Tenant{
		ID:           id,
		Name:         "Test Store",
		Slug:         "test-store",
		BusinessType: domain.BusinessTypeRetail,
		Plan:         "starter",
		Country:      "ID",
		Currency:     "IDR",
		Timezone:     "Asia/Jakarta",
		IsActive:     true,
	}
}

func fakeOutlet(tenantID uuid.UUID) *domain.Outlet {
	return &domain.Outlet{
		ID:            uuid.New(),
		TenantID:      tenantID,
		Name:          "Main Outlet",
		Code:          "MAIN",
		IsActive:      true,
		IsMainOutlet:  true,
		OpenTime:      "08:00",
		CloseTime:     "22:00",
		TaxRate:       0.11,
		ServiceCharge: 0,
	}
}

// ──────────────────────────────────────────────
// GET /v1/tenants/me
// ──────────────────────────────────────────────

func TestGetMyTenants_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("GetMyTenants", mock.Anything, userID).Return([]*domain.Tenant{fakeTenant(tenantID)}, nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "GET", "/v1/tenants/me", nil)

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
	data := body["data"].([]interface{})
	assert.Len(t, data, 1)
}

func TestGetMyTenants_InvalidUserID(t *testing.T) {
	uc := &mockTenantUsecase{}
	app := setupApp(uc, "not-a-uuid")

	status, body := doRequest(app, "GET", "/v1/tenants/me", nil)
	assert.Equal(t, 401, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "UNAUTHORIZED", errBody["code"])
}

// ──────────────────────────────────────────────
// GET /v1/tenants/:tenant_id
// ──────────────────────────────────────────────

func TestGetTenant_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("GetTenant", mock.Anything, tenantID).Return(fakeTenant(tenantID), nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "GET", "/v1/tenants/"+tenantID.String(), nil)

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "Test Store", data["name"])
}

func TestGetTenant_NotFound(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("GetTenant", mock.Anything, tenantID).Return(nil, domain.ErrTenantNotFound)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "GET", "/v1/tenants/"+tenantID.String(), nil)

	assert.Equal(t, 404, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "TENANT_NOT_FOUND", errBody["code"])
}

func TestGetTenant_InvalidID(t *testing.T) {
	uc := &mockTenantUsecase{}
	app := setupApp(uc, uuid.New().String())

	status, body := doRequest(app, "GET", "/v1/tenants/not-a-uuid", nil)
	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_PARAM", errBody["code"])
}

// ──────────────────────────────────────────────
// PATCH /v1/tenants/:tenant_id
// ──────────────────────────────────────────────

func TestUpdateTenant_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	updated := fakeTenant(tenantID)
	updated.Name = "Updated Store"
	uc.On("UpdateTenant", mock.Anything, tenantID, domain.UpdateTenantInput{Name: "Updated Store"}).
		Return(updated, nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "PATCH", "/v1/tenants/"+tenantID.String(), map[string]interface{}{
		"name": "Updated Store",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "Updated Store", data["name"])
}

// ──────────────────────────────────────────────
// GET /v1/tenants/:tenant_id/outlets
// ──────────────────────────────────────────────

func TestGetOutlets_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("GetOutlets", mock.Anything, tenantID).Return([]*domain.Outlet{fakeOutlet(tenantID)}, nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "GET", "/v1/tenants/"+tenantID.String()+"/outlets", nil)

	assert.Equal(t, 200, status)
	data := body["data"].([]interface{})
	assert.Len(t, data, 1)
}

// ──────────────────────────────────────────────
// POST /v1/tenants/:tenant_id/outlets
// ──────────────────────────────────────────────

func TestCreateOutlet_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	outlet := fakeOutlet(tenantID)
	uc.On("CreateOutlet", mock.Anything, tenantID, mock.MatchedBy(func(i domain.CreateOutletInput) bool {
		return i.Name == "Branch 1" && i.Code == "BR1"
	})).Return(outlet, nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "POST", "/v1/tenants/"+tenantID.String()+"/outlets", map[string]interface{}{
		"name": "Branch 1",
		"code": "BR1",
	})

	assert.Equal(t, 201, status)
	assert.True(t, body["success"].(bool))
}

func TestCreateOutlet_PlanLimitReached(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("CreateOutlet", mock.Anything, tenantID, mock.Anything).Return(nil, domain.ErrMaxOutletsReached)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "POST", "/v1/tenants/"+tenantID.String()+"/outlets", map[string]interface{}{
		"name": "Branch 2",
		"code": "BR2",
	})

	assert.Equal(t, 402, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "PLAN_LIMIT_OUTLETS", errBody["code"])
}

func TestCreateOutlet_ValidationError(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	app := setupApp(uc, userID.String())

	// Missing required "code" field
	status, _ := doRequest(app, "POST", "/v1/tenants/"+tenantID.String()+"/outlets", map[string]interface{}{
		"name": "Branch 1",
		// code missing
	})
	assert.Equal(t, 422, status)
	uc.AssertNotCalled(t, "CreateOutlet")
}

// ──────────────────────────────────────────────
// DELETE /v1/tenants/:tenant_id/outlets/:outlet_id
// ──────────────────────────────────────────────

func TestDeleteOutlet_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	outletID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("DeleteOutlet", mock.Anything, tenantID, outletID).Return(nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "DELETE",
		"/v1/tenants/"+tenantID.String()+"/outlets/"+outletID.String(), nil)

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

// ──────────────────────────────────────────────
// GET /v1/tenants/:tenant_id/members
// ──────────────────────────────────────────────

func TestGetMembers_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("GetMembers", mock.Anything, tenantID).Return([]*domain.TenantUser{
		{ID: uuid.New(), TenantID: tenantID, UserID: userID, IsActive: true},
	}, nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "GET", "/v1/tenants/"+tenantID.String()+"/members", nil)

	require.Equal(t, 200, status)
	data := body["data"].([]interface{})
	assert.Len(t, data, 1)
}

// ──────────────────────────────────────────────
// POST /v1/tenants/:tenant_id/invitations
// ──────────────────────────────────────────────

func TestInviteUser_Success(t *testing.T) {
	inviterID := uuid.New()
	tenantID := uuid.New()
	roleID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("InviteUser", mock.Anything, tenantID, inviterID, mock.MatchedBy(func(i domain.InviteUserInput) bool {
		return i.Email == "newuser@example.com" && i.RoleID == roleID
	})).Return(nil)

	app := setupApp(uc, inviterID.String())
	status, body := doRequest(app, "POST", "/v1/tenants/"+tenantID.String()+"/invitations",
		map[string]interface{}{
			"email":   "newuser@example.com",
			"role_id": roleID.String(),
		})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestInviteUser_PlanLimitReached(t *testing.T) {
	inviterID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("InviteUser", mock.Anything, tenantID, inviterID, mock.Anything).Return(domain.ErrMaxUsersReached)

	app := setupApp(uc, inviterID.String())
	status, body := doRequest(app, "POST", "/v1/tenants/"+tenantID.String()+"/invitations",
		map[string]interface{}{
			"email":   "newuser@example.com",
			"role_id": uuid.New().String(),
		})

	assert.Equal(t, 402, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "PLAN_LIMIT_USERS", errBody["code"])
}

// ──────────────────────────────────────────────
// POST /v1/invitations/accept
// ──────────────────────────────────────────────

func TestAcceptInvitation_Success(t *testing.T) {
	userID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("AcceptInvitation", mock.Anything, userID, "valid-token").Return(nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "POST", "/v1/invitations/accept", map[string]interface{}{
		"token": "valid-token",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestAcceptInvitation_InvalidToken(t *testing.T) {
	userID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("AcceptInvitation", mock.Anything, userID, "bad-token").Return(domain.ErrInvitationNotFound)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "POST", "/v1/invitations/accept", map[string]interface{}{
		"token": "bad-token",
	})

	assert.Equal(t, 404, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVITATION_NOT_FOUND", errBody["code"])
}

// ──────────────────────────────────────────────
// DELETE /v1/tenants/:tenant_id/members/:user_id
// ──────────────────────────────────────────────

func TestRemoveMember_Success(t *testing.T) {
	requesterID := uuid.New()
	tenantID := uuid.New()
	targetID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("RemoveMember", mock.Anything, tenantID, requesterID, targetID).Return(nil)

	app := setupApp(uc, requesterID.String())
	status, body := doRequest(app, "DELETE",
		"/v1/tenants/"+tenantID.String()+"/members/"+targetID.String(), nil)

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestRemoveMember_CannotRemoveOwner(t *testing.T) {
	requesterID := uuid.New()
	tenantID := uuid.New()
	ownerID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("RemoveMember", mock.Anything, tenantID, requesterID, ownerID).Return(domain.ErrCannotDeleteOwner)

	app := setupApp(uc, requesterID.String())
	status, body := doRequest(app, "DELETE",
		"/v1/tenants/"+tenantID.String()+"/members/"+ownerID.String(), nil)

	assert.Equal(t, 403, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "CANNOT_DELETE_OWNER", errBody["code"])
}

// ──────────────────────────────────────────────
// GET /v1/tenants/:tenant_id/roles
// ──────────────────────────────────────────────

func TestGetRoles_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("GetRoles", mock.Anything, tenantID).Return([]*domain.Role{
		{ID: uuid.New(), Name: "Owner", Slug: "owner", IsSystem: true},
		{ID: uuid.New(), Name: "Staff", Slug: "staff", IsSystem: true},
	}, nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "GET", "/v1/tenants/"+tenantID.String()+"/roles", nil)

	assert.Equal(t, 200, status)
	data := body["data"].([]interface{})
	assert.Len(t, data, 2)
}

// ──────────────────────────────────────────────
// PATCH /v1/tenants/:tenant_id/outlets/:outlet_id
// ──────────────────────────────────────────────

func TestUpdateOutlet_Success(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	outletID := uuid.New()
	uc := &mockTenantUsecase{}
	updated := fakeOutlet(tenantID)
	updated.Name = "Updated Outlet"
	uc.On("UpdateOutlet", mock.Anything, tenantID, outletID, mock.Anything).Return(updated, nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "PATCH",
		"/v1/tenants/"+tenantID.String()+"/outlets/"+outletID.String(),
		map[string]interface{}{"name": "Updated Outlet", "code": "UPD"})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestUpdateOutlet_InvalidTenantID(t *testing.T) {
	uc := &mockTenantUsecase{}
	app := setupApp(uc, uuid.New().String())

	status, body := doRequest(app, "PATCH", "/v1/tenants/bad-id/outlets/"+uuid.New().String(), nil)
	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_PARAM", errBody["code"])
}

func TestUpdateOutlet_InvalidOutletID(t *testing.T) {
	uc := &mockTenantUsecase{}
	app := setupApp(uc, uuid.New().String())

	status, body := doRequest(app, "PATCH", "/v1/tenants/"+uuid.New().String()+"/outlets/bad-id", nil)
	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_PARAM", errBody["code"])
}

// ──────────────────────────────────────────────
// PATCH /v1/tenants/:tenant_id/members/:user_id/role
// ──────────────────────────────────────────────

func TestUpdateMemberRole_Success(t *testing.T) {
	requesterID := uuid.New()
	tenantID := uuid.New()
	memberID := uuid.New()
	newRoleID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("UpdateMemberRole", mock.Anything, tenantID, memberID, newRoleID).Return(nil)

	app := setupApp(uc, requesterID.String())
	status, body := doRequest(app, "PATCH",
		"/v1/tenants/"+tenantID.String()+"/members/"+memberID.String()+"/role",
		map[string]interface{}{"role_id": newRoleID.String()})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

// ──────────────────────────────────────────────
// Additional error case coverage
// ──────────────────────────────────────────────

func TestGetOutlets_NotFound(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("GetOutlets", mock.Anything, tenantID).Return(nil, domain.ErrTenantNotFound)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "GET", "/v1/tenants/"+tenantID.String()+"/outlets", nil)

	assert.Equal(t, 404, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "TENANT_NOT_FOUND", errBody["code"])
}

func TestDeleteOutlet_InvalidOutletID(t *testing.T) {
	uc := &mockTenantUsecase{}
	app := setupApp(uc, uuid.New().String())

	status, body := doRequest(app, "DELETE",
		"/v1/tenants/"+uuid.New().String()+"/outlets/not-a-uuid", nil)
	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_PARAM", errBody["code"])
}

func TestRemoveMember_InvalidMemberID(t *testing.T) {
	uc := &mockTenantUsecase{}
	app := setupApp(uc, uuid.New().String())

	status, body := doRequest(app, "DELETE",
		"/v1/tenants/"+uuid.New().String()+"/members/not-a-uuid", nil)
	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_PARAM", errBody["code"])
}

func TestGetMyTenants_Empty(t *testing.T) {
	userID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("GetMyTenants", mock.Anything, userID).Return([]*domain.Tenant{}, nil)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "GET", "/v1/tenants/me", nil)

	assert.Equal(t, 200, status)
	data := body["data"].([]interface{})
	assert.Len(t, data, 0)
}

func TestCreateOutlet_OutletCodeExists(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("CreateOutlet", mock.Anything, tenantID, mock.Anything).Return(nil, domain.ErrOutletCodeExists)

	app := setupApp(uc, userID.String())
	status, body := doRequest(app, "POST", "/v1/tenants/"+tenantID.String()+"/outlets",
		map[string]interface{}{"name": "Dup Outlet", "code": "MAIN"})

	assert.Equal(t, 409, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "OUTLET_CODE_CONFLICT", errBody["code"])
}

// ──────────────────────────────────────────────
// POST /internal/tenants
// ──────────────────────────────────────────────

func TestCreateTenantInternal_Success(t *testing.T) {
	ownerID := uuid.New()
	uc := &mockTenantUsecase{}
	uc.On("CreateTenant", mock.Anything, ownerID, domain.CreateTenantInput{
		OwnerUserID:  ownerID,
		BusinessName: "Kopi Susu Segar",
		BusinessType: domain.BusinessTypeFnB,
	}).Return(fakeTenant(uuid.New()), nil)

	app := setupApp(uc, uuid.New().String())
	status, body := doRequest(app, "POST", "/internal/tenants", map[string]interface{}{
		"owner_user_id": ownerID.String(),
		"business_name": "Kopi Susu Segar",
		"business_type": "fnb",
	})

	assert.Equal(t, 201, status)
	assert.True(t, body["success"].(bool))
}

func TestCreateTenantInternal_ValidationError(t *testing.T) {
	uc := &mockTenantUsecase{}
	app := setupApp(uc, uuid.New().String())

	// Missing required fields
	status, _ := doRequest(app, "POST", "/internal/tenants", map[string]interface{}{
		"business_name": "Test",
		// missing owner_user_id and business_type
	})
	assert.Equal(t, 422, status)
	uc.AssertNotCalled(t, "CreateTenant")
}
