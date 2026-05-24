package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/extendedsynaptic/xynpos/auth-service/internal/delivery/http/handler"
	"github.com/extendedsynaptic/xynpos/auth-service/internal/domain"
)

// ──────────────────────────────────────────────
// Mock AuthUsecase
// ──────────────────────────────────────────────

type mockAuthUsecase struct{ mock.Mock }

func (m *mockAuthUsecase) Register(ctx context.Context, input domain.RegisterInput) (*domain.TokenPair, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenPair), args.Error(1)
}

func (m *mockAuthUsecase) Login(ctx context.Context, input domain.LoginInput, ip string) (*domain.TokenPair, error) {
	args := m.Called(ctx, input, ip)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenPair), args.Error(1)
}

func (m *mockAuthUsecase) RefreshToken(ctx context.Context, input domain.RefreshInput) (*domain.TokenPair, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenPair), args.Error(1)
}

func (m *mockAuthUsecase) ForgotPassword(ctx context.Context, input domain.ForgotPasswordInput) error {
	return m.Called(ctx, input).Error(0)
}

func (m *mockAuthUsecase) ResetPassword(ctx context.Context, input domain.ResetPasswordInput) error {
	return m.Called(ctx, input).Error(0)
}

func (m *mockAuthUsecase) ResendOTP(ctx context.Context, input domain.ResendOTPInput) error {
	return m.Called(ctx, input).Error(0)
}

func (m *mockAuthUsecase) VerifyEmail(ctx context.Context, userID uuid.UUID, input domain.VerifyEmailInput) error {
	return m.Called(ctx, userID, input).Error(0)
}

func (m *mockAuthUsecase) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *mockAuthUsecase) UpdateProfile(ctx context.Context, userID uuid.UUID, input domain.UpdateProfileInput) (*domain.User, error) {
	args := m.Called(ctx, userID, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *mockAuthUsecase) ChangePassword(ctx context.Context, userID uuid.UUID, input domain.ChangePasswordInput) error {
	return m.Called(ctx, userID, input).Error(0)
}

func (m *mockAuthUsecase) Logout(ctx context.Context, refreshToken string) error {
	return m.Called(ctx, refreshToken).Error(0)
}

func (m *mockAuthUsecase) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*domain.Session, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Session), args.Error(1)
}

func (m *mockAuthUsecase) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	return m.Called(ctx, userID, sessionID).Error(0)
}

func (m *mockAuthUsecase) ValidateToken(ctx context.Context, accessToken string) (*domain.TokenClaims, error) {
	args := m.Called(ctx, accessToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TokenClaims), args.Error(1)
}

// ──────────────────────────────────────────────
// Test setup helpers
// ──────────────────────────────────────────────

func setupApp(uc domain.AuthUsecase) *fiber.App {
	app := fiber.New(fiber.Config{
		// Suppress internal error logs during tests
	})
	h := handler.NewAuthHandler(uc)

	// No-op auth middleware for protected tests (inject locals manually)
	noopAuth := func(c fiber.Ctx) error {
		return c.Next()
	}
	h.Register(app, noopAuth)
	return app
}

func setupAppWithAuth(uc domain.AuthUsecase, userID string) *fiber.App {
	app := fiber.New()
	h := handler.NewAuthHandler(uc)

	authMW := func(c fiber.Ctx) error {
		c.Locals("userID", userID)
		c.Locals("tenantID", uuid.New().String())
		return c.Next()
	}
	h.Register(app, authMW)
	return app
}

func doRequest(app *fiber.App, method, path string, body interface{}) *httptest.ResponseRecorder {
	var b io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		b = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, b)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, _ := app.Test(req)
	return &httptest.ResponseRecorder{Code: resp.StatusCode}
}

func doRequestFull(app *fiber.App, method, path string, body interface{}) (int, map[string]interface{}) {
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

func fakeTokenPair() *domain.TokenPair {
	return &domain.TokenPair{
		AccessToken:  "fake.access.token",
		RefreshToken: "fake-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}
}

// ──────────────────────────────────────────────
// POST /v1/auth/register
// ──────────────────────────────────────────────

func TestHandleRegister_Success(t *testing.T) {
	uc := &mockAuthUsecase{}
	uc.On("Register", mock.Anything, mock.MatchedBy(func(i domain.RegisterInput) bool {
		return i.Email == "new@example.com"
	})).Return(fakeTokenPair(), nil)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/register", map[string]interface{}{
		"name":          "Test User",
		"email":         "new@example.com",
		"password":      "SecurePass123!",
		"business_name": "My Business",
		"business_type": "retail",
	})

	assert.Equal(t, 201, status)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "fake.access.token", data["access_token"])
}

func TestHandleRegister_ValidationError_MissingEmail(t *testing.T) {
	uc := &mockAuthUsecase{}
	app := setupApp(uc)

	status, body := doRequestFull(app, "POST", "/v1/auth/register", map[string]interface{}{
		"name":     "Test User",
		"password": "SecurePass123!",
		// email is missing
	})

	assert.Equal(t, 422, status)
	assert.False(t, body["success"].(bool))
	uc.AssertNotCalled(t, "Register")
}

func TestHandleRegister_EmailAlreadyExists(t *testing.T) {
	uc := &mockAuthUsecase{}
	uc.On("Register", mock.Anything, mock.Anything).Return(nil, domain.ErrEmailAlreadyExists)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/register", map[string]interface{}{
		"name":          "Test User",
		"email":         "existing@example.com",
		"password":      "SecurePass123!",
		"business_name": "My Business",
		"business_type": "retail",
	})

	assert.Equal(t, 409, status)
	assert.False(t, body["success"].(bool))
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "EMAIL_CONFLICT", errBody["code"])
}

// ──────────────────────────────────────────────
// POST /v1/auth/login
// ──────────────────────────────────────────────

func TestLogin_Handler_Success(t *testing.T) {
	uc := &mockAuthUsecase{}
	uc.On("Login", mock.Anything, mock.MatchedBy(func(i domain.LoginInput) bool {
		return i.Email == "test@example.com"
	}), mock.Anything).Return(fakeTokenPair(), nil)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/login", map[string]interface{}{
		"email":    "test@example.com",
		"password": "password123",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestLogin_Handler_InvalidCredentials(t *testing.T) {
	uc := &mockAuthUsecase{}
	uc.On("Login", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, domain.ErrInvalidCredentials)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/login", map[string]interface{}{
		"email":    "wrong@example.com",
		"password": "wrongpassword",
	})

	assert.Equal(t, 401, status)
	assert.False(t, body["success"].(bool))
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_CREDENTIALS", errBody["code"])
}

func TestLogin_Handler_MissingFields(t *testing.T) {
	uc := &mockAuthUsecase{}
	app := setupApp(uc)

	// No body at all
	status, _ := doRequestFull(app, "POST", "/v1/auth/login", map[string]interface{}{})

	assert.Equal(t, 422, status)
	uc.AssertNotCalled(t, "Login")
}

// ──────────────────────────────────────────────
// POST /v1/auth/refresh
// ──────────────────────────────────────────────

func TestRefresh_Handler_Success(t *testing.T) {
	uc := &mockAuthUsecase{}
	newPair := &domain.TokenPair{
		AccessToken:  "new.access.token",
		RefreshToken: "new-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}
	uc.On("RefreshToken", mock.Anything, mock.MatchedBy(func(i domain.RefreshInput) bool {
		return i.RefreshToken == "valid-refresh-token"
	})).Return(newPair, nil)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/refresh", map[string]interface{}{
		"refresh_token": "valid-refresh-token",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "new.access.token", data["access_token"])
}

func TestRefresh_Handler_TokenReused(t *testing.T) {
	uc := &mockAuthUsecase{}
	uc.On("RefreshToken", mock.Anything, mock.Anything).
		Return(nil, domain.ErrRefreshTokenReused)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/refresh", map[string]interface{}{
		"refresh_token": "stolen-token",
	})

	assert.Equal(t, 401, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "TOKEN_REUSED", errBody["code"])
}

// ──────────────────────────────────────────────
// POST /v1/auth/forgot-password (ALWAYS returns 200)
// ──────────────────────────────────────────────

func TestForgotPassword_Handler_AlwaysReturns200(t *testing.T) {
	uc := &mockAuthUsecase{}
	// Even if error is returned (unknown email), handler always returns 200
	uc.On("ForgotPassword", mock.Anything, mock.Anything).
		Return(domain.ErrAccountNotFound)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/forgot-password", map[string]interface{}{
		"email": "unknown@example.com",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

// ──────────────────────────────────────────────
// GET /v1/auth/me (protected)
// ──────────────────────────────────────────────

func TestGetProfile_Handler_Success(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}
	uc.On("GetProfile", mock.Anything, userID).Return(&domain.User{
		ID:            userID,
		Email:         "test@example.com",
		Name:          "Test User",
		EmailVerified: true,
		IsActive:      true,
	}, nil)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "GET", "/v1/auth/me", nil)

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "test@example.com", data["email"])
	assert.Equal(t, "Test User", data["name"])
}

func TestGetProfile_Handler_InvalidUserID(t *testing.T) {
	uc := &mockAuthUsecase{}

	// Inject invalid UUID
	app := fiber.New()
	h := handler.NewAuthHandler(uc)
	authMW := func(c fiber.Ctx) error {
		c.Locals("userID", "not-a-valid-uuid")
		return c.Next()
	}
	h.Register(app, authMW)

	status, body := doRequestFull(app, "GET", "/v1/auth/me", nil)
	assert.Equal(t, 401, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "UNAUTHORIZED", errBody["code"])
	uc.AssertNotCalled(t, "GetProfile")
}

// ──────────────────────────────────────────────
// POST /v1/auth/verify-email (protected)
// ──────────────────────────────────────────────

func TestVerifyEmail_Handler_Success(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}
	uc.On("VerifyEmail", mock.Anything, userID, domain.VerifyEmailInput{OTP: "123456"}).
		Return(nil)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "POST", "/v1/auth/verify-email", map[string]interface{}{
		"otp": "123456",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestVerifyEmail_Handler_InvalidOTP(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}
	uc.On("VerifyEmail", mock.Anything, userID, mock.Anything).
		Return(domain.ErrOTPNotFound)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "POST", "/v1/auth/verify-email", map[string]interface{}{
		"otp": "999999",
	})

	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_OTP", errBody["code"])
}

// ──────────────────────────────────────────────
// POST /v1/auth/logout (protected)
// ──────────────────────────────────────────────

func TestLogout_Handler_Success(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}
	uc.On("Logout", mock.Anything, "old-refresh-token").Return(nil)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "POST", "/v1/auth/logout", map[string]interface{}{
		"refresh_token": "old-refresh-token",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

// ──────────────────────────────────────────────
// GET /v1/auth/sessions (protected)
// ──────────────────────────────────────────────

func TestGetSessions_Handler_Success(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}

	now := time.Now()
	sessions := []*domain.Session{
		{
			ID:         uuid.New(),
			DeviceName: "Chrome on MacOS",
			IPAddress:  "127.0.0.1",
			ExpiresAt:  now.Add(24 * time.Hour),
			IsCurrent:  true,
		},
	}
	uc.On("GetActiveSessions", mock.Anything, userID).Return(sessions, nil)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "GET", "/v1/auth/sessions", nil)

	require.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

// ──────────────────────────────────────────────
// POST /v1/auth/reset-password
// ──────────────────────────────────────────────

func TestResetPassword_Handler_Success(t *testing.T) {
	uc := &mockAuthUsecase{}
	uc.On("ResetPassword", mock.Anything, mock.MatchedBy(func(i domain.ResetPasswordInput) bool {
		return i.OTP == "123456" && i.NewPassword == "NewPass123!"
	})).Return(nil)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/reset-password", map[string]interface{}{
		"email":        "test@example.com",
		"otp":          "123456",
		"new_password": "NewPass123!",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestResetPassword_Handler_InvalidOTP(t *testing.T) {
	uc := &mockAuthUsecase{}
	uc.On("ResetPassword", mock.Anything, mock.Anything).Return(domain.ErrOTPNotFound)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/reset-password", map[string]interface{}{
		"email":        "test@example.com",
		"otp":          "000000",
		"new_password": "NewPass123!",
	})

	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_OTP", errBody["code"])
}

// ──────────────────────────────────────────────
// DELETE /v1/auth/sessions/:session_id (protected)
// ──────────────────────────────────────────────

func TestRevokeSession_Handler_Success(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	uc := &mockAuthUsecase{}
	uc.On("RevokeSession", mock.Anything, userID, sessionID).Return(nil)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "DELETE", "/v1/auth/sessions/"+sessionID.String(), nil)

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestRevokeSession_Handler_InvalidSessionID(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}
	app := setupAppWithAuth(uc, userID.String())

	status, body := doRequestFull(app, "DELETE", "/v1/auth/sessions/not-a-uuid", nil)
	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_PARAM", errBody["code"])
}

// ──────────────────────────────────────────────
// POST /v1/auth/resend-otp
// ──────────────────────────────────────────────

func TestResendOTP_Handler_Success(t *testing.T) {
	uc := &mockAuthUsecase{}
	uc.On("ResendOTP", mock.Anything, mock.MatchedBy(func(i domain.ResendOTPInput) bool {
		return i.Email == "user@example.com" && i.Type == domain.OTPTypeEmailVerification
	})).Return(nil)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/resend-otp", map[string]interface{}{
		"email": "user@example.com",
		"type":  "email_verification",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestResendOTP_Handler_RateLimited(t *testing.T) {
	uc := &mockAuthUsecase{}
	uc.On("ResendOTP", mock.Anything, mock.Anything).Return(domain.ErrTooManyOTPRequests)

	app := setupApp(uc)
	status, body := doRequestFull(app, "POST", "/v1/auth/resend-otp", map[string]interface{}{
		"email": "user@example.com",
		"type":  "email_verification",
	})

	assert.Equal(t, 429, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "TOO_MANY_REQUESTS", errBody["code"])
}

// ──────────────────────────────────────────────
// PATCH /v1/auth/me (protected)
// ──────────────────────────────────────────────

func TestUpdateProfile_Handler_Success(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}
	uc.On("UpdateProfile", mock.Anything, userID, domain.UpdateProfileInput{
		Name: "Updated Name",
	}).Return(&domain.User{
		ID:    userID,
		Email: "test@example.com",
		Name:  "Updated Name",
	}, nil)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "PATCH", "/v1/auth/me", map[string]interface{}{
		"name": "Updated Name",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "Updated Name", data["name"])
}

// ──────────────────────────────────────────────
// POST /v1/auth/change-password (protected)
// ──────────────────────────────────────────────

func TestChangePassword_Handler_Success(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}
	uc.On("ChangePassword", mock.Anything, userID, domain.ChangePasswordInput{
		OldPassword: "OldPass123!",
		NewPassword: "NewPass456!",
	}).Return(nil)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "POST", "/v1/auth/change-password", map[string]interface{}{
		"old_password": "OldPass123!",
		"new_password": "NewPass456!",
	})

	assert.Equal(t, 200, status)
	assert.True(t, body["success"].(bool))
}

func TestChangePassword_Handler_WrongOldPassword(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}
	uc.On("ChangePassword", mock.Anything, userID, mock.Anything).Return(domain.ErrInvalidOldPassword)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "POST", "/v1/auth/change-password", map[string]interface{}{
		"old_password": "WrongPass!",
		"new_password": "NewPass456!",
	})

	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_PASSWORD", errBody["code"])
}

func TestChangePassword_Handler_SamePassword(t *testing.T) {
	userID := uuid.New()
	uc := &mockAuthUsecase{}
	uc.On("ChangePassword", mock.Anything, userID, mock.Anything).Return(domain.ErrSamePassword)

	app := setupAppWithAuth(uc, userID.String())
	status, body := doRequestFull(app, "POST", "/v1/auth/change-password", map[string]interface{}{
		"old_password": "SamePass123!",
		"new_password": "SamePass123!",
	})

	assert.Equal(t, 400, status)
	errBody := body["error"].(map[string]interface{})
	assert.Equal(t, "SAME_PASSWORD", errBody["code"])
}
