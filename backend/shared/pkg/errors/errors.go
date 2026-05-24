package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// ──────────────────────────────────────────────
// Base domain error type
// ──────────────────────────────────────────────

// AppError represents a structured application error with an error code,
// HTTP status, and an optional cause.
type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
	Cause      error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// New creates a new AppError.
func New(code, message string, httpStatus int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: httpStatus}
}

// Wrap creates an AppError wrapping a cause.
func Wrap(err error, code, message string, httpStatus int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: httpStatus, Cause: err}
}

// ──────────────────────────────────────────────
// Sentinel domain errors (shared across services)
// ──────────────────────────────────────────────

var (
	// Auth
	ErrUnauthorized    = New("UNAUTHORIZED", "Authentication required", http.StatusUnauthorized)
	ErrForbidden       = New("FORBIDDEN", "You do not have permission to perform this action", http.StatusForbidden)
	ErrTokenExpired    = New("TOKEN_EXPIRED", "Access token has expired", http.StatusUnauthorized)
	ErrTokenInvalid    = New("TOKEN_INVALID", "Access token is invalid", http.StatusUnauthorized)
	ErrTokenReused     = New("TOKEN_REUSED", "Refresh token reuse detected — all sessions revoked", http.StatusUnauthorized)
	ErrInvalidPassword = New("INVALID_PASSWORD", "Password is incorrect", http.StatusUnauthorized)

	// Tenant
	ErrTenantNotFound  = New("TENANT_NOT_FOUND", "Tenant not found", http.StatusNotFound)
	ErrTenantSuspended = New("TENANT_SUSPENDED", "Tenant account is suspended", http.StatusForbidden)
	ErrPlanLimitReached= New("PLAN_LIMIT_REACHED", "Your current plan does not support this feature", http.StatusPaymentRequired)

	// User
	ErrUserNotFound    = New("USER_NOT_FOUND", "User not found", http.StatusNotFound)
	ErrEmailConflict   = New("EMAIL_CONFLICT", "An account with this email already exists", http.StatusConflict)
	ErrPhoneConflict   = New("PHONE_CONFLICT", "An account with this phone number already exists", http.StatusConflict)
	ErrInvalidOTP      = New("INVALID_OTP", "OTP is invalid or expired", http.StatusBadRequest)

	// General
	ErrNotFound        = New("NOT_FOUND", "Resource not found", http.StatusNotFound)
	ErrConflict        = New("CONFLICT", "Resource already exists", http.StatusConflict)
	ErrBadRequest      = New("BAD_REQUEST", "Invalid request", http.StatusBadRequest)
	ErrInternal        = New("INTERNAL_ERROR", "An internal error occurred", http.StatusInternalServerError)
	ErrServiceUnavail  = New("SERVICE_UNAVAILABLE", "Service temporarily unavailable", http.StatusServiceUnavailable)
	ErrRateLimited     = New("RATE_LIMITED", "Too many requests, please try again later", http.StatusTooManyRequests)

	// Resource-specific (services will add more in their own domain/errors.go)
	ErrProductNotFound     = New("PRODUCT_NOT_FOUND", "Product not found", http.StatusNotFound)
	ErrInsufficientStock   = New("INSUFFICIENT_STOCK", "Insufficient stock for this operation", http.StatusUnprocessableEntity)
	ErrOutOfStock          = New("OUT_OF_STOCK", "Product is out of stock", http.StatusUnprocessableEntity)
	ErrTransactionNotFound = New("TRANSACTION_NOT_FOUND", "Transaction not found", http.StatusNotFound)
	ErrInvalidPayment      = New("INVALID_PAYMENT_AMOUNT", "Payment amount does not cover the total", http.StatusUnprocessableEntity)
	ErrTransactionVoided   = New("TRANSACTION_VOIDED", "Transaction has already been voided", http.StatusConflict)
	ErrCustomerNotFound    = New("CUSTOMER_NOT_FOUND", "Customer not found", http.StatusNotFound)
	ErrOutletNotFound      = New("OUTLET_NOT_FOUND", "Outlet not found", http.StatusNotFound)
)

// ──────────────────────────────────────────────
// HTTP status resolution
// ──────────────────────────────────────────────

// HTTPStatus returns the appropriate HTTP status code for a given error.
// It traverses the error chain to find an *AppError.
func HTTPStatus(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus
	}
	return http.StatusInternalServerError
}

// Code returns the error code string.
func Code(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return "INTERNAL_ERROR"
}

// Message returns the user-facing error message.
func Message(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "An internal error occurred"
}

// Is checks if target is in the error chain (wraps std errors.Is).
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in the chain assignable to target (wraps std errors.As).
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}
