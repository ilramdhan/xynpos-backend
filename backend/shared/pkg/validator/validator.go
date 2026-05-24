package validator

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

var (
	instance *validator.Validate
	once     sync.Once

	// Pre-compiled regex for custom validators
	rePhoneID   = regexp.MustCompile(`^(\+62|62|0)[0-9]{8,13}$`)
	reAlphaNum  = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	reHTMLTags  = regexp.MustCompile(`<[^>]*>`)
	reScriptTag = regexp.MustCompile(`(?i)<script|javascript:|on\w+\s*=`)
)

// ValidationError represents a single field validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// Get returns the singleton validator instance with all custom validators registered.
func Get() *validator.Validate {
	once.Do(func() {
		instance = validator.New()

		// Use JSON tag names instead of struct field names in error messages
		instance.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := fld.Tag.Get("json")
			if name == "" || name == "-" {
				return ""
			}
			// Handle "name,omitempty" → "name"
			if idx := strings.Index(name, ","); idx != -1 {
				name = name[:idx]
			}
			return name
		})

		// Custom validators
		_ = instance.RegisterValidation("xss", validateNoXSS)
		_ = instance.RegisterValidation("phone_id", validatePhoneID)
		_ = instance.RegisterValidation("uuid4", validateUUID4)
		_ = instance.RegisterValidation("currency_idr", validateCurrencyIDR)
		_ = instance.RegisterValidation("slug", validateSlug)
	})
	return instance
}

// Validate validates a struct and returns human-readable ValidationErrors.
// Returns nil if the struct is valid.
func Validate(v interface{}) []ValidationError {
	err := Get().Struct(v)
	if err == nil {
		return nil
	}
	return mapErrors(err)
}

// mapErrors converts validator errors to []ValidationError.
func mapErrors(err error) []ValidationError {
	var errs []ValidationError
	for _, fe := range err.(validator.ValidationErrors) {
		errs = append(errs, ValidationError{
			Field:   fe.Field(),
			Message: humanMessage(fe),
			Value:   fmt.Sprintf("%v", fe.Value()),
		})
	}
	return errs
}

func humanMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fe.Field() + " is required"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", fe.Field(), fe.Param())
	case "email":
		return fe.Field() + " must be a valid email address"
	case "uuid4":
		return fe.Field() + " must be a valid UUID"
	case "xss":
		return fe.Field() + " contains disallowed characters"
	case "phone_id":
		return fe.Field() + " must be a valid Indonesian phone number"
	case "currency_idr":
		return fe.Field() + " must be a positive currency amount"
	case "url":
		return fe.Field() + " must be a valid URL"
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", fe.Field(), fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", fe.Field(), fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", fe.Field(), fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", fe.Field(), fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", fe.Field(), fe.Param())
	default:
		return fmt.Sprintf("%s failed validation: %s", fe.Field(), fe.Tag())
	}
}

// ──────────────────────────────────────────────
// Custom validators
// ──────────────────────────────────────────────

// validateNoXSS rejects HTML tags and inline JS.
func validateNoXSS(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	if reHTMLTags.MatchString(val) {
		return false
	}
	if reScriptTag.MatchString(val) {
		return false
	}
	return true
}

// validatePhoneID validates Indonesian phone numbers.
// Accepts: +628xxx, 628xxx, 08xxx
func validatePhoneID(fl validator.FieldLevel) bool {
	return rePhoneID.MatchString(fl.Field().String())
}

// validateUUID4 validates a UUID v4.
func validateUUID4(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	if val == "" {
		return true // respect "omitempty"
	}
	_, err := uuid.Parse(val)
	return err == nil
}

// validateCurrencyIDR validates a positive IDR currency amount.
func validateCurrencyIDR(fl validator.FieldLevel) bool {
	val := fl.Field().Float()
	return val >= 0
}

// validateSlug validates a URL-safe slug.
func validateSlug(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	for _, ch := range val {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return false
		}
	}
	return len(val) > 0
}
