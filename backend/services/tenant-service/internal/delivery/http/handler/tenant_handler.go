package handler

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/extendedsynaptic/xynpos/tenant-service/internal/domain"
	apperrors "github.com/extendedsynaptic/xynpos/shared/pkg/errors"
	"github.com/extendedsynaptic/xynpos/shared/pkg/logger"
	"github.com/extendedsynaptic/xynpos/shared/pkg/middleware"
	"github.com/extendedsynaptic/xynpos/shared/pkg/response"
	"github.com/extendedsynaptic/xynpos/shared/pkg/validator"

	"github.com/google/uuid"
)

// TenantHandler handles HTTP requests for tenant management.
type TenantHandler struct {
	usecase domain.TenantUsecase
}

// NewTenantHandler creates a new TenantHandler.
func NewTenantHandler(uc domain.TenantUsecase) *TenantHandler {
	return &TenantHandler{usecase: uc}
}

// Register registers all HTTP routes for tenant-service.
func (h *TenantHandler) Register(app *fiber.App, authMW fiber.Handler) {
	v1 := app.Group("/v1", authMW)

	// Tenant CRUD
	v1.Get("/tenants/me", h.GetMyTenants)
	v1.Get("/tenants/:tenant_id", h.GetTenant)
	v1.Patch("/tenants/:tenant_id", h.UpdateTenant)

	// Outlets
	v1.Get("/tenants/:tenant_id/outlets", h.GetOutlets)
	v1.Post("/tenants/:tenant_id/outlets", h.CreateOutlet)
	v1.Patch("/tenants/:tenant_id/outlets/:outlet_id", h.UpdateOutlet)
	v1.Delete("/tenants/:tenant_id/outlets/:outlet_id", h.DeleteOutlet)

	// Members
	v1.Get("/tenants/:tenant_id/members", h.GetMembers)
	v1.Post("/tenants/:tenant_id/invitations", h.InviteUser)
	v1.Delete("/tenants/:tenant_id/members/:user_id", h.RemoveMember)
	v1.Patch("/tenants/:tenant_id/members/:user_id/role", h.UpdateMemberRole)

	// Roles
	v1.Get("/tenants/:tenant_id/roles", h.GetRoles)

	// Invitations (public-ish: requires auth but no tenant scope)
	v1.Post("/invitations/accept", h.AcceptInvitation)

	// Internal route for auth-service to create tenant on register
	// Protected by internal API key middleware (not public JWT)
	app.Post("/internal/tenants", h.CreateTenantInternal)
}

// ──────────────────────────────────────────────
// Internal Endpoints
// ──────────────────────────────────────────────

// CreateTenantInternal handles POST /internal/tenants
//
//	@Summary		Create tenant (internal)
//	@Description	Called by auth-service on user registration. Protected by internal API key.
//	@Tags			Internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		domain.CreateTenantInput	true	"Tenant data"
//	@Success		201		{object}	response.SuccessResponse
//	@Failure		400		{object}	response.ErrorResponse
//	@Router			/internal/tenants [post]
func (h *TenantHandler) CreateTenantInternal(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())

	var input domain.CreateTenantInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}
	if ve := validator.Validate(input); ve != nil {
		return c.Status(422).JSON(response.ValidationErrors(toResponseValidationErrors(ve)))
	}

	tenant, err := h.usecase.CreateTenant(c.Context(), input.OwnerUserID, input)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(201).JSON(response.Success(toTenantResponse(tenant)))
}

// ──────────────────────────────────────────────
// Tenant Endpoints
// ──────────────────────────────────────────────

// GetMyTenants handles GET /v1/tenants/me
//
//	@Summary		Get my tenants
//	@Description	Returns all tenants the authenticated user belongs to.
//	@Tags			Tenants
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{object}	response.SuccessResponse
//	@Router			/v1/tenants/me [get]
func (h *TenantHandler) GetMyTenants(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	userID, err := parseUUID(middleware.GetUserID(c))
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	tenants, err := h.usecase.GetMyTenants(c.Context(), userID)
	if err != nil {
		return h.handleError(c, log, err)
	}

	out := make([]*TenantResponse, len(tenants))
	for i, t := range tenants {
		out[i] = toTenantResponse(t)
	}
	return c.Status(200).JSON(response.Success(out))
}

// GetTenant handles GET /v1/tenants/:tenant_id
//
//	@Summary		Get tenant by ID
//	@Tags			Tenants
//	@Security		BearerAuth
//	@Produce		json
//	@Param			tenant_id	path		string	true	"Tenant UUID"
//	@Success		200			{object}	response.SuccessResponse
//	@Failure		403			{object}	response.ErrorResponse
//	@Failure		404			{object}	response.ErrorResponse
//	@Router			/v1/tenants/{tenant_id} [get]
func (h *TenantHandler) GetTenant(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}

	tenant, err := h.usecase.GetTenant(c.Context(), tenantID)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(toTenantResponse(tenant)))
}

// UpdateTenant handles PATCH /v1/tenants/:tenant_id
//
//	@Summary		Update tenant profile
//	@Tags			Tenants
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			tenant_id	path		string						true	"Tenant UUID"
//	@Param			body		body		domain.UpdateTenantInput	true	"Update data"
//	@Success		200			{object}	response.SuccessResponse
//	@Failure		404			{object}	response.ErrorResponse
//	@Router			/v1/tenants/{tenant_id} [patch]
func (h *TenantHandler) UpdateTenant(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}

	var input domain.UpdateTenantInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	tenant, err := h.usecase.UpdateTenant(c.Context(), tenantID, input)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(toTenantResponse(tenant)))
}

// ──────────────────────────────────────────────
// Outlet Endpoints
// ──────────────────────────────────────────────

// GetOutlets handles GET /v1/tenants/:tenant_id/outlets
//
//	@Summary		List outlets
//	@Tags			Outlets
//	@Security		BearerAuth
//	@Produce		json
//	@Param			tenant_id	path		string	true	"Tenant UUID"
//	@Success		200			{object}	response.SuccessResponse
//	@Router			/v1/tenants/{tenant_id}/outlets [get]
func (h *TenantHandler) GetOutlets(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}

	outlets, err := h.usecase.GetOutlets(c.Context(), tenantID)
	if err != nil {
		return h.handleError(c, log, err)
	}

	out := make([]*OutletResponse, len(outlets))
	for i, o := range outlets {
		out[i] = toOutletResponse(o)
	}
	return c.Status(200).JSON(response.Success(out))
}

// CreateOutlet handles POST /v1/tenants/:tenant_id/outlets
//
//	@Summary		Create outlet
//	@Tags			Outlets
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			tenant_id	path		string					true	"Tenant UUID"
//	@Param			body		body		domain.CreateOutletInput	true	"Outlet data"
//	@Success		201			{object}	response.SuccessResponse
//	@Failure		422			{object}	response.ErrorResponse
//	@Failure		409			{object}	response.ErrorResponse
//	@Router			/v1/tenants/{tenant_id}/outlets [post]
func (h *TenantHandler) CreateOutlet(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}

	var input domain.CreateOutletInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}
	if ve := validator.Validate(input); ve != nil {
		return c.Status(422).JSON(response.ValidationErrors(toResponseValidationErrors(ve)))
	}

	outlet, err := h.usecase.CreateOutlet(c.Context(), tenantID, input)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(201).JSON(response.Success(toOutletResponse(outlet)))
}

// UpdateOutlet handles PATCH /v1/tenants/:tenant_id/outlets/:outlet_id
//
//	@Summary		Update outlet
//	@Tags			Outlets
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			tenant_id	path		string					true	"Tenant UUID"
//	@Param			outlet_id	path		string					true	"Outlet UUID"
//	@Param			body		body		domain.CreateOutletInput	true	"Outlet data"
//	@Success		200			{object}	response.SuccessResponse
//	@Router			/v1/tenants/{tenant_id}/outlets/{outlet_id} [patch]
func (h *TenantHandler) UpdateOutlet(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}
	outletID, err := parseUUID(c.Params("outlet_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid outlet ID"))
	}

	var input domain.CreateOutletInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	outlet, err := h.usecase.UpdateOutlet(c.Context(), tenantID, outletID, input)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(toOutletResponse(outlet)))
}

// DeleteOutlet handles DELETE /v1/tenants/:tenant_id/outlets/:outlet_id
//
//	@Summary		Delete outlet
//	@Tags			Outlets
//	@Security		BearerAuth
//	@Produce		json
//	@Param			tenant_id	path		string	true	"Tenant UUID"
//	@Param			outlet_id	path		string	true	"Outlet UUID"
//	@Success		200			{object}	response.SuccessResponse
//	@Failure		403			{object}	response.ErrorResponse
//	@Router			/v1/tenants/{tenant_id}/outlets/{outlet_id} [delete]
func (h *TenantHandler) DeleteOutlet(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}
	outletID, err := parseUUID(c.Params("outlet_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid outlet ID"))
	}

	if err := h.usecase.DeleteOutlet(c.Context(), tenantID, outletID); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(nil))
}

// ──────────────────────────────────────────────
// Member Endpoints
// ──────────────────────────────────────────────

// GetMembers handles GET /v1/tenants/:tenant_id/members
//
//	@Summary		List tenant members
//	@Tags			Members
//	@Security		BearerAuth
//	@Produce		json
//	@Param			tenant_id	path		string	true	"Tenant UUID"
//	@Success		200			{object}	response.SuccessResponse
//	@Router			/v1/tenants/{tenant_id}/members [get]
func (h *TenantHandler) GetMembers(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}

	members, err := h.usecase.GetMembers(c.Context(), tenantID)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(members))
}

// InviteUser handles POST /v1/tenants/:tenant_id/invitations
//
//	@Summary		Invite user to tenant
//	@Tags			Members
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			tenant_id	path		string					true	"Tenant UUID"
//	@Param			body		body		domain.InviteUserInput	true	"Invitation data"
//	@Success		200			{object}	response.SuccessResponse
//	@Failure		422			{object}	response.ErrorResponse
//	@Router			/v1/tenants/{tenant_id}/invitations [post]
func (h *TenantHandler) InviteUser(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}
	inviterID, err := parseUUID(middleware.GetUserID(c))
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	var input domain.InviteUserInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}
	if ve := validator.Validate(input); ve != nil {
		return c.Status(422).JSON(response.ValidationErrors(toResponseValidationErrors(ve)))
	}

	if err := h.usecase.InviteUser(c.Context(), tenantID, inviterID, input); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(map[string]string{
		"message": "Invitation sent successfully",
	}))
}

// AcceptInvitation handles POST /v1/invitations/accept
//
//	@Summary		Accept invitation
//	@Tags			Members
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		acceptInvitationRequest	true	"Invitation token"
//	@Success		200		{object}	response.SuccessResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Router			/v1/invitations/accept [post]
type acceptInvitationRequest struct {
	Token string `json:"token" validate:"required"`
}

func (h *TenantHandler) AcceptInvitation(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	userID, err := parseUUID(middleware.GetUserID(c))
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	var req acceptInvitationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}
	if ve := validator.Validate(req); ve != nil {
		return c.Status(422).JSON(response.ValidationErrors(toResponseValidationErrors(ve)))
	}

	if err := h.usecase.AcceptInvitation(c.Context(), userID, req.Token); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(map[string]string{
		"message": "Successfully joined the tenant",
	}))
}

// RemoveMember handles DELETE /v1/tenants/:tenant_id/members/:user_id
//
//	@Summary		Remove member
//	@Tags			Members
//	@Security		BearerAuth
//	@Produce		json
//	@Param			tenant_id	path		string	true	"Tenant UUID"
//	@Param			user_id		path		string	true	"User UUID to remove"
//	@Success		200			{object}	response.SuccessResponse
//	@Failure		403			{object}	response.ErrorResponse
//	@Router			/v1/tenants/{tenant_id}/members/{user_id} [delete]
func (h *TenantHandler) RemoveMember(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}
	targetUserID, err := parseUUID(c.Params("user_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid user ID"))
	}
	requesterID, err := parseUUID(middleware.GetUserID(c))
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	if err := h.usecase.RemoveMember(c.Context(), tenantID, requesterID, targetUserID); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(nil))
}

// UpdateMemberRole handles PATCH /v1/tenants/:tenant_id/members/:user_id/role
//
//	@Summary		Update member role
//	@Tags			Members
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			tenant_id	path		string					true	"Tenant UUID"
//	@Param			user_id		path		string					true	"User UUID"
//	@Param			body		body		updateMemberRoleRequest	true	"New role"
//	@Success		200			{object}	response.SuccessResponse
//	@Router			/v1/tenants/{tenant_id}/members/{user_id}/role [patch]
type updateMemberRoleRequest struct {
	RoleID string `json:"role_id" validate:"required,uuid4"`
}

func (h *TenantHandler) UpdateMemberRole(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}
	userID, err := parseUUID(c.Params("user_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid user ID"))
	}

	var req updateMemberRoleRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}
	if ve := validator.Validate(req); ve != nil {
		return c.Status(422).JSON(response.ValidationErrors(toResponseValidationErrors(ve)))
	}

	roleID, err := parseUUID(req.RoleID)
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid role ID"))
	}

	if err := h.usecase.UpdateMemberRole(c.Context(), tenantID, userID, roleID); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(nil))
}

// ──────────────────────────────────────────────
// Role Endpoints
// ──────────────────────────────────────────────

// GetRoles handles GET /v1/tenants/:tenant_id/roles
//
//	@Summary		List roles
//	@Tags			Roles
//	@Security		BearerAuth
//	@Produce		json
//	@Param			tenant_id	path		string	true	"Tenant UUID"
//	@Success		200			{object}	response.SuccessResponse
//	@Router			/v1/tenants/{tenant_id}/roles [get]
func (h *TenantHandler) GetRoles(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	tenantID, err := parseUUID(c.Params("tenant_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid tenant ID"))
	}

	roles, err := h.usecase.GetRoles(c.Context(), tenantID)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(roles))
}

// ──────────────────────────────────────────────
// Error mapping
// ──────────────────────────────────────────────

func (h *TenantHandler) handleError(c fiber.Ctx, log *zap.Logger, err error) error {
	switch {
	case errors.Is(err, domain.ErrTenantNotFound):
		return c.Status(404).JSON(response.Error("TENANT_NOT_FOUND", err.Error()))
	case errors.Is(err, domain.ErrOutletNotFound):
		return c.Status(404).JSON(response.Error("OUTLET_NOT_FOUND", err.Error()))
	case errors.Is(err, domain.ErrRoleNotFound):
		return c.Status(404).JSON(response.Error("ROLE_NOT_FOUND", err.Error()))
	case errors.Is(err, domain.ErrInvitationNotFound):
		return c.Status(404).JSON(response.Error("INVITATION_NOT_FOUND", err.Error()))
	case errors.Is(err, domain.ErrSlugAlreadyExists):
		return c.Status(409).JSON(response.Error("SLUG_CONFLICT", err.Error()))
	case errors.Is(err, domain.ErrOutletCodeExists):
		return c.Status(409).JSON(response.Error("OUTLET_CODE_CONFLICT", err.Error()))
	case errors.Is(err, domain.ErrAlreadyMember):
		return c.Status(409).JSON(response.Error("ALREADY_MEMBER", err.Error()))
	case errors.Is(err, domain.ErrMaxOutletsReached):
		return c.Status(402).JSON(response.Error("PLAN_LIMIT_OUTLETS", err.Error()))
	case errors.Is(err, domain.ErrMaxUsersReached):
		return c.Status(402).JSON(response.Error("PLAN_LIMIT_USERS", err.Error()))
	case errors.Is(err, domain.ErrNotTenantMember):
		return c.Status(403).JSON(response.Error("NOT_TENANT_MEMBER", err.Error()))
	case errors.Is(err, domain.ErrCannotDeleteOwner):
		return c.Status(403).JSON(response.Error("CANNOT_DELETE_OWNER", err.Error()))
	case errors.Is(err, domain.ErrSystemRole):
		return c.Status(403).JSON(response.Error("SYSTEM_ROLE", err.Error()))
	}

	httpErr := apperrors.ToHTTPError(err)
	if httpErr.StatusCode >= 500 {
		log.Error("internal error in tenant handler", zap.Error(err))
	}
	return c.Status(httpErr.StatusCode).JSON(response.Error(httpErr.Code, httpErr.Message))
}

// ──────────────────────────────────────────────
// Response DTOs
// ──────────────────────────────────────────────

// TenantResponse is the public-safe tenant representation.
type TenantResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	BusinessType string `json:"business_type"`
	Plan         string `json:"plan"`
	LogoURL      string `json:"logo_url,omitempty"`
	Website      string `json:"website,omitempty"`
	Address      string `json:"address,omitempty"`
	City         string `json:"city,omitempty"`
	Province     string `json:"province,omitempty"`
	Country      string `json:"country"`
	Currency     string `json:"currency"`
	Timezone     string `json:"timezone"`
	IsActive     bool   `json:"is_active"`
}

// OutletResponse is the public-safe outlet representation.
type OutletResponse struct {
	ID            string  `json:"id"`
	TenantID      string  `json:"tenant_id"`
	Name          string  `json:"name"`
	Code          string  `json:"code"`
	Phone         string  `json:"phone,omitempty"`
	Address       string  `json:"address,omitempty"`
	City          string  `json:"city,omitempty"`
	Province      string  `json:"province,omitempty"`
	IsActive      bool    `json:"is_active"`
	IsMainOutlet  bool    `json:"is_main_outlet"`
	OpenTime      string  `json:"open_time"`
	CloseTime     string  `json:"close_time"`
	TaxRate       float64 `json:"tax_rate"`
	TaxIncluded   bool    `json:"tax_included"`
	ServiceCharge float64 `json:"service_charge"`
}

func toTenantResponse(t *domain.Tenant) *TenantResponse {
	return &TenantResponse{
		ID:           t.ID.String(),
		Name:         t.Name,
		Slug:         t.Slug,
		BusinessType: string(t.BusinessType),
		Plan:         t.Plan,
		LogoURL:      t.LogoURL,
		Website:      t.Website,
		Address:      t.Address,
		City:         t.City,
		Province:     t.Province,
		Country:      t.Country,
		Currency:     t.Currency,
		Timezone:     t.Timezone,
		IsActive:     t.IsActive,
	}
}

func toOutletResponse(o *domain.Outlet) *OutletResponse {
	return &OutletResponse{
		ID:            o.ID.String(),
		TenantID:      o.TenantID.String(),
		Name:          o.Name,
		Code:          o.Code,
		Phone:         o.Phone,
		Address:       o.Address,
		City:          o.City,
		Province:      o.Province,
		IsActive:      o.IsActive,
		IsMainOutlet:  o.IsMainOutlet,
		OpenTime:      o.OpenTime,
		CloseTime:     o.CloseTime,
		TaxRate:       o.TaxRate,
		TaxIncluded:   o.TaxIncluded,
		ServiceCharge: o.ServiceCharge,
	}
}

// helpers

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func toResponseValidationErrors(ve []validator.ValidationError) []response.ValidationError {
	out := make([]response.ValidationError, len(ve))
	for i, v := range ve {
		out[i] = response.ValidationError{
			Field:   v.Field,
			Message: v.Message,
			Value:   v.Value,
		}
	}
	return out
}
