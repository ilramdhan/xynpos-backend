package response

import "net/http"

// ──────────────────────────────────────────────
// Response types
// ──────────────────────────────────────────────

// SuccessResponse is the standard envelope for single-resource responses.
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

// ListResponse is the standard envelope for list/paginated responses.
type ListResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// ErrorResponse is the standard envelope for error responses.
type ErrorResponse struct {
	Success bool      `json:"success"`
	Error   ErrorBody `json:"error"`
}

// ErrorBody holds machine-readable and human-readable error information.
type ErrorBody struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	HTTPStatus int               `json:"http_status"`
	Details    []ValidationError `json:"details,omitempty"`
}

// Meta holds pagination metadata.
type Meta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// ValidationError represents a single field validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ──────────────────────────────────────────────
// Constructors
// ──────────────────────────────────────────────

// Success builds a standard success response.
func Success(data interface{}) SuccessResponse {
	return SuccessResponse{Success: true, Data: data}
}

// SuccessList builds a paginated list response.
func SuccessList(data interface{}, meta *Meta) ListResponse {
	return ListResponse{Success: true, Data: data, Meta: meta}
}

// Error builds an error response.
func Error(code, message string) ErrorResponse {
	return ErrorResponse{
		Success: false,
		Error: ErrorBody{
			Code:       code,
			Message:    message,
			HTTPStatus: codeToHTTPStatus(code),
		},
	}
}

// ErrorWithStatus builds an error response with an explicit HTTP status override.
func ErrorWithStatus(httpStatus int, code, message string) ErrorResponse {
	return ErrorResponse{
		Success: false,
		Error: ErrorBody{
			Code:       code,
			Message:    message,
			HTTPStatus: httpStatus,
		},
	}
}

// ValidationErrors builds a validation error response.
func ValidationErrors(errs []ValidationError) ErrorResponse {
	return ErrorResponse{
		Success: false,
		Error: ErrorBody{
			Code:       "VALIDATION_ERROR",
			Message:    "Request validation failed",
			HTTPStatus: http.StatusBadRequest,
			Details:    errs,
		},
	}
}

// NewMeta builds pagination Meta from page, perPage and total count.
func NewMeta(page, perPage int, total int64) *Meta {
	totalPages := int(total) / perPage
	if int(total)%perPage != 0 {
		totalPages++
	}
	return &Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// ──────────────────────────────────────────────
// Error code → HTTP status mapping
// ──────────────────────────────────────────────

func codeToHTTPStatus(code string) int {
	m := map[string]int{
		"UNAUTHORIZED":         http.StatusUnauthorized,
		"FORBIDDEN":            http.StatusForbidden,
		"NOT_FOUND":            http.StatusNotFound,
		"CONFLICT":             http.StatusConflict,
		"VALIDATION_ERROR":     http.StatusBadRequest,
		"RATE_LIMITED":         http.StatusTooManyRequests,
		"PLAN_LIMIT_REACHED":   http.StatusPaymentRequired,
		"TENANT_SUSPENDED":     http.StatusForbidden,
		"INTERNAL_ERROR":       http.StatusInternalServerError,
		"SERVICE_UNAVAILABLE":  http.StatusServiceUnavailable,
		"UNPROCESSABLE_ENTITY": http.StatusUnprocessableEntity,
		"BAD_GATEWAY":          http.StatusBadGateway,
	}
	if status, ok := m[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}
