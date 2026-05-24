package handler

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"

	"github.com/extendedsynaptic/xynpos/auth-service/internal/domain"
	apperrors "github.com/extendedsynaptic/xynpos/shared/pkg/errors"
	"github.com/extendedsynaptic/xynpos/shared/pkg/logger"
	"github.com/extendedsynaptic/xynpos/shared/pkg/middleware"
	"github.com/extendedsynaptic/xynpos/shared/pkg/response"
	"github.com/extendedsynaptic/xynpos/shared/pkg/validator"
)

// AuthHandler handles HTTP requests for the auth service.
type AuthHandler struct {
	usecase domain.AuthUsecase
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(uc domain.AuthUsecase) *AuthHandler {
	return &AuthHandler{usecase: uc}
}

// Register registers all HTTP routes.
func (h *AuthHandler) Register(app *fiber.App, authMW fiber.Handler) {
	v1 := app.Group("/v1/auth")

	// Public routes (no JWT)
	v1.Post("/register", h.HandleRegister)
	v1.Post("/login", h.Login)
	v1.Post("/refresh", h.Refresh)
	v1.Post("/forgot-password", h.ForgotPassword)
	v1.Post("/reset-password", h.ResetPassword)
	v1.Post("/resend-otp", h.ResendOTP)

	// Protected routes (JWT required)
	v1.Post("/verify-email", authMW, h.VerifyEmail)
	v1.Get("/me", authMW, h.GetProfile)
	v1.Patch("/me", authMW, h.UpdateProfile)
	v1.Post("/change-password", authMW, h.ChangePassword)
	v1.Post("/logout", authMW, h.Logout)
	v1.Get("/sessions", authMW, h.GetSessions)
	v1.Delete("/sessions/:session_id", authMW, h.RevokeSession)
}

// ──────────────────────────────────────────────

// HandleRegister handles POST /v1/auth/register
//
//	@Summary	Register new tenant and owner account
//	@Tags		Auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		domain.RegisterInput	true	"Registration data"
//	@Success	201		{object}	response.SuccessResponse
//	@Failure	400		{object}	response.ErrorResponse
//	@Failure	409		{object}	response.ErrorResponse
//	@Router		/v1/auth/register [post]
func (h *AuthHandler) HandleRegister(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())

	var input domain.RegisterInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	if ve := validator.Validate(input); ve != nil {
		return c.Status(422).JSON(response.ValidationErrors(toResponseValidationErrors(ve)))
	}

	tokens, err := h.usecase.Register(c.Context(), input)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(201).JSON(response.Success(tokens))
}

// Login handles POST /v1/auth/login
//
//	@Summary	Login with email and password
//	@Tags		Auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		domain.LoginInput	true	"Login credentials"
//	@Success	200		{object}	response.SuccessResponse
//	@Failure	401		{object}	response.ErrorResponse
//	@Router		/v1/auth/login [post]
func (h *AuthHandler) Login(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())

	var input domain.LoginInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	if ve := validator.Validate(input); ve != nil {
		return c.Status(422).JSON(response.ValidationErrors(toResponseValidationErrors(ve)))
	}

	tokens, err := h.usecase.Login(c.Context(), input, c.IP())
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(tokens))
}

// Refresh handles POST /v1/auth/refresh
//
//	@Summary	Refresh access token
//	@Tags		Auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		domain.RefreshInput	true	"Refresh token"
//	@Success	200		{object}	response.SuccessResponse
//	@Failure	401		{object}	response.ErrorResponse
//	@Router		/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())

	var input domain.RefreshInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	if ve := validator.Validate(input); ve != nil {
		return c.Status(422).JSON(response.ValidationErrors(toResponseValidationErrors(ve)))
	}

	tokens, err := h.usecase.RefreshToken(c.Context(), input)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(tokens))
}

// ForgotPassword handles POST /v1/auth/forgot-password
//
//	@Summary	Request password reset OTP
//	@Tags		Auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		domain.ForgotPasswordInput	true	"Email address"
//	@Success	200		{object}	response.SuccessResponse
//	@Router		/v1/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c fiber.Ctx) error {
	var input domain.ForgotPasswordInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}
	// Always succeed — prevent email enumeration
	_ = h.usecase.ForgotPassword(c.Context(), input)
	return c.Status(200).JSON(response.Success(nil))
}

// ResetPassword handles POST /v1/auth/reset-password
//
//	@Summary	Reset password using OTP
//	@Tags		Auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		domain.ResetPasswordInput	true	"OTP and new password"
//	@Success	200		{object}	response.SuccessResponse
//	@Failure	400		{object}	response.ErrorResponse
//	@Router		/v1/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())

	var input domain.ResetPasswordInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	if ve := validator.Validate(input); ve != nil {
		return c.Status(422).JSON(response.ValidationErrors(toResponseValidationErrors(ve)))
	}

	if err := h.usecase.ResetPassword(c.Context(), input); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(nil))
}

// ResendOTP handles POST /v1/auth/resend-otp
//
//	@Summary	Resend OTP code
//	@Tags		Auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		domain.ResendOTPInput	true	"Email and OTP type"
//	@Success	200		{object}	response.SuccessResponse
//	@Router		/v1/auth/resend-otp [post]
func (h *AuthHandler) ResendOTP(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())

	var input domain.ResendOTPInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	if err := h.usecase.ResendOTP(c.Context(), input); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(nil))
}

// VerifyEmail handles POST /v1/auth/verify-email
//
//	@Summary		Verify email with OTP
//	@Tags			Auth
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		domain.VerifyEmailInput	true	"OTP code"
//	@Success		200		{object}	response.SuccessResponse
//	@Failure		400		{object}	response.ErrorResponse
//	@Router			/v1/auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	userID := middleware.GetUserID(c)

	var input domain.VerifyEmailInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	uid, err := parseUUID(userID)
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	if err := h.usecase.VerifyEmail(c.Context(), uid, input); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(nil))
}

// GetProfile handles GET /v1/auth/me
//
//	@Summary		Get current user profile
//	@Tags			Auth
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{object}	response.SuccessResponse
//	@Router			/v1/auth/me [get]
func (h *AuthHandler) GetProfile(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	userID := middleware.GetUserID(c)

	uid, err := parseUUID(userID)
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	user, err := h.usecase.GetProfile(c.Context(), uid)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(toProfileResponse(user)))
}

// UpdateProfile handles PATCH /v1/auth/me
//
//	@Summary		Update user profile
//	@Tags			Auth
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		domain.UpdateProfileInput	true	"Profile fields to update"
//	@Success		200		{object}	response.SuccessResponse
//	@Router			/v1/auth/me [patch]
func (h *AuthHandler) UpdateProfile(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	userID := middleware.GetUserID(c)

	uid, err := parseUUID(userID)
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	var input domain.UpdateProfileInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	user, err := h.usecase.UpdateProfile(c.Context(), uid, input)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(toProfileResponse(user)))
}

// ChangePassword handles POST /v1/auth/change-password
//
//	@Summary		Change user password
//	@Tags			Auth
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		domain.ChangePasswordInput	true	"Old and new password"
//	@Success		200		{object}	response.SuccessResponse
//	@Router			/v1/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	userID := middleware.GetUserID(c)

	uid, err := parseUUID(userID)
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	var input domain.ChangePasswordInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	if err := h.usecase.ChangePassword(c.Context(), uid, input); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(nil))
}

// Logout handles POST /v1/auth/logout
//
//	@Summary		Logout and revoke refresh token
//	@Tags			Auth
//	@Security		BearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		domain.RefreshInput	true	"Refresh token to revoke"
//	@Success		200		{object}	response.SuccessResponse
//	@Router			/v1/auth/logout [post]
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())

	var input domain.RefreshInput
	if err := c.Bind().JSON(&input); err != nil {
		return c.Status(400).JSON(response.Error("INVALID_BODY", "Invalid request body"))
	}

	if err := h.usecase.Logout(c.Context(), input.RefreshToken); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(nil))
}

// GetSessions handles GET /v1/auth/sessions
//
//	@Summary		List active sessions
//	@Tags			Auth
//	@Security		BearerAuth
//	@Produce		json
//	@Success		200	{object}	response.SuccessResponse
//	@Router			/v1/auth/sessions [get]
func (h *AuthHandler) GetSessions(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	userID := middleware.GetUserID(c)

	uid, err := parseUUID(userID)
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	sessions, err := h.usecase.GetActiveSessions(c.Context(), uid)
	if err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(sessions))
}

// RevokeSession handles DELETE /v1/auth/sessions/:session_id
//
//	@Summary		Revoke a specific session
//	@Tags			Auth
//	@Security		BearerAuth
//	@Produce		json
//	@Param			session_id	path		string	true	"Session ID"
//	@Success		200			{object}	response.SuccessResponse
//	@Router			/v1/auth/sessions/{session_id} [delete]
func (h *AuthHandler) RevokeSession(c fiber.Ctx) error {
	log := logger.FromContext(c.Context())
	userID := middleware.GetUserID(c)

	uid, err := parseUUID(userID)
	if err != nil {
		return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Invalid user context"))
	}

	sessionID, err := parseUUID(c.Params("session_id"))
	if err != nil {
		return c.Status(400).JSON(response.Error("INVALID_PARAM", "Invalid session ID"))
	}

	if err := h.usecase.RevokeSession(c.Context(), uid, sessionID); err != nil {
		return h.handleError(c, log, err)
	}

	return c.Status(200).JSON(response.Success(nil))
}

// handleError maps errors to appropriate HTTP responses.
func (h *AuthHandler) handleError(c fiber.Ctx, log *zap.Logger, err error) error {
	// Map auth domain errors to HTTP status + code
	switch {
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return c.Status(409).JSON(response.Error("EMAIL_CONFLICT", err.Error()))
	case errors.Is(err, domain.ErrPhoneAlreadyExists):
		return c.Status(409).JSON(response.Error("PHONE_CONFLICT", err.Error()))
	case errors.Is(err, domain.ErrInvalidCredentials):
		return c.Status(401).JSON(response.Error("INVALID_CREDENTIALS", "Invalid email or password"))
	case errors.Is(err, domain.ErrAccountSuspended):
		return c.Status(403).JSON(response.Error("ACCOUNT_SUSPENDED", err.Error()))
	case errors.Is(err, domain.ErrAccountNotFound):
		return c.Status(404).JSON(response.Error("USER_NOT_FOUND", err.Error()))
	case errors.Is(err, domain.ErrRefreshTokenNotFound), errors.Is(err, domain.ErrRefreshTokenExpired):
		return c.Status(401).JSON(response.Error("TOKEN_INVALID", "Refresh token is invalid or expired"))
	case errors.Is(err, domain.ErrRefreshTokenRevoked):
		return c.Status(401).JSON(response.Error("TOKEN_REVOKED", err.Error()))
	case errors.Is(err, domain.ErrRefreshTokenReused):
		return c.Status(401).JSON(response.Error("TOKEN_REUSED", err.Error()))
	case errors.Is(err, domain.ErrOTPNotFound):
		return c.Status(400).JSON(response.Error("INVALID_OTP", "OTP is invalid or expired"))
	case errors.Is(err, domain.ErrTooManyOTPRequests):
		return c.Status(429).JSON(response.Error("TOO_MANY_REQUESTS", err.Error()))
	case errors.Is(err, domain.ErrInvalidOldPassword):
		return c.Status(400).JSON(response.Error("INVALID_PASSWORD", err.Error()))
	case errors.Is(err, domain.ErrSamePassword):
		return c.Status(400).JSON(response.Error("SAME_PASSWORD", err.Error()))
	case errors.Is(err, domain.ErrEmailNotVerified):
		return c.Status(403).JSON(response.Error("EMAIL_NOT_VERIFIED", err.Error()))
	}

	// Fall back to AppError chain resolution
	httpErr := apperrors.ToHTTPError(err)
	if httpErr.StatusCode >= 500 {
		log.Error("internal error in auth handler", zap.Error(err))
	}
	return c.Status(httpErr.StatusCode).JSON(response.Error(httpErr.Code, httpErr.Message))
}


// ──────────────────────────────────────────────
// Response DTOs
// ──────────────────────────────────────────────

// ProfileResponse is the public-safe user profile (no password hash).
type ProfileResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Phone         string `json:"phone,omitempty"`
	Name          string `json:"name"`
	AvatarURL     string `json:"avatar_url,omitempty"`
	EmailVerified bool   `json:"email_verified"`
	IsActive      bool   `json:"is_active"`
}

func toProfileResponse(u *domain.User) *ProfileResponse {
	return &ProfileResponse{
		ID:            u.ID.String(),
		Email:         u.Email,
		Phone:         u.Phone,
		Name:          u.Name,
		AvatarURL:     u.AvatarURL,
		EmailVerified: u.EmailVerified,
		IsActive:      u.IsActive,
	}
}

// toResponseValidationErrors converts []validator.ValidationError to []response.ValidationError
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
