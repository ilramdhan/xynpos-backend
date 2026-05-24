package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	apperrors "github.com/extendedsynaptic/xynpos/shared/pkg/errors"
	"github.com/extendedsynaptic/xynpos/shared/pkg/jwt"
	"github.com/extendedsynaptic/xynpos/shared/pkg/response"
)

// contextKey type for Fiber locals to avoid collisions.
type contextKey string

const (
	LocalKeyTenantID    = "tenantID"
	LocalKeyUserID      = "userID"
	LocalKeyOutletID    = "outletID"
	LocalKeyRole        = "role"
	LocalKeyPermissions = "permissions"
	LocalKeyPlan        = "plan"
	LocalKeyClaims      = "claims"
)

// RequireAuth returns a Fiber middleware that validates the JWT access token.
// It sets tenant, user, role, permissions, and plan into Fiber locals.
//
// Usage:
//
//	app.Get("/v1/products", middleware.RequireAuth(jwtManager), handler.List)
func RequireAuth(jwtMgr *jwt.Manager) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Authorization header is required"))
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Authorization header must be 'Bearer <token>'"))
		}

		claims, err := jwtMgr.ParseAccessToken(parts[1])
		if err != nil {
			if apperrors.Is(err, apperrors.ErrTokenExpired) {
				return c.Status(401).JSON(response.Error("TOKEN_EXPIRED", "Access token has expired"))
			}
			return c.Status(401).JSON(response.Error("TOKEN_INVALID", "Access token is invalid"))
		}

		// Inject into Fiber locals (accessible in handlers via c.Locals)
		c.Locals(LocalKeyClaims, claims)
		c.Locals(LocalKeyTenantID, claims.TenantID)
		c.Locals(LocalKeyUserID, claims.UserID)
		c.Locals(LocalKeyOutletID, claims.OutletID)
		c.Locals(LocalKeyRole, claims.Role)
		c.Locals(LocalKeyPermissions, claims.Permissions)
		c.Locals(LocalKeyPlan, claims.Plan)

		return c.Next()
	}
}

// GetTenantID retrieves the tenant ID from Fiber locals.
// ALWAYS use this — NEVER read tenantID from request body or query params.
func GetTenantID(c fiber.Ctx) string {
	v, _ := c.Locals(LocalKeyTenantID).(string)
	return v
}

// GetUserID retrieves the user ID from Fiber locals.
func GetUserID(c fiber.Ctx) string {
	v, _ := c.Locals(LocalKeyUserID).(string)
	return v
}

// GetOutletID retrieves the outlet ID from Fiber locals.
func GetOutletID(c fiber.Ctx) string {
	v, _ := c.Locals(LocalKeyOutletID).(string)
	return v
}

// GetRole retrieves the user's role from Fiber locals.
func GetRole(c fiber.Ctx) string {
	v, _ := c.Locals(LocalKeyRole).(string)
	return v
}

// GetPermissions retrieves the user's permission list from Fiber locals.
func GetPermissions(c fiber.Ctx) []string {
	v, _ := c.Locals(LocalKeyPermissions).([]string)
	return v
}

// GetPlan retrieves the tenant's subscription plan from Fiber locals.
func GetPlan(c fiber.Ctx) string {
	v, _ := c.Locals(LocalKeyPlan).(string)
	return v
}

// GetClaims retrieves the full JWT claims from Fiber locals.
func GetClaims(c fiber.Ctx) *jwt.Claims {
	v, _ := c.Locals(LocalKeyClaims).(*jwt.Claims)
	return v
}

// RequirePermission returns a middleware that checks if the authenticated user
// has the required permission string.
//
// Usage:
//
//	app.Get("/v1/reports/sales", middleware.RequireAuth(...), middleware.RequirePermission("report:read"), handler)
func RequirePermission(permission string) fiber.Handler {
	return func(c fiber.Ctx) error {
		perms := GetPermissions(c)
		for _, p := range perms {
			if p == permission || p == "*" {
				return c.Next()
			}
		}
		return c.Status(403).JSON(response.Error("FORBIDDEN",
			"You do not have permission: "+permission))
	}
}

// RequirePlan returns a middleware that ensures the tenant's plan meets a minimum level.
// Plans are ordered: free < starter < pro < business < enterprise.
func RequirePlan(minPlan string) fiber.Handler {
	return func(c fiber.Ctx) error {
		tenantPlan := GetPlan(c)
		if !isPlanSufficient(tenantPlan, minPlan) {
			return c.Status(402).JSON(response.Error("PLAN_LIMIT_REACHED",
				"This feature requires the '"+minPlan+"' plan or higher"))
		}
		return c.Next()
	}
}

// RequireRole returns a middleware that ensures the user has one of the allowed roles.
func RequireRole(roles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		userRole := GetRole(c)
		for _, r := range roles {
			if r == userRole {
				return c.Next()
			}
		}
		return c.Status(403).JSON(response.Error("FORBIDDEN",
			"Your role does not have access to this resource"))
	}
}

// planOrder maps plan names to their ordinal value.
var planOrder = map[string]int{
	"free":       0,
	"starter":    1,
	"pro":        2,
	"business":   3,
	"enterprise": 4,
}

func isPlanSufficient(tenantPlan, requiredPlan string) bool {
	return planOrder[tenantPlan] >= planOrder[requiredPlan]
}
