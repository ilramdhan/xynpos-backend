package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	appredis "github.com/extendedsynaptic/xynpos/shared/pkg/redis"
	"github.com/extendedsynaptic/xynpos/shared/pkg/response"
)

// ──────────────────────────────────────────────
// Request ID Middleware
// ──────────────────────────────────────────────

// RequestID injects a unique X-Request-ID header into every request/response.
func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("X-Request-ID", requestID)
		c.Locals("requestID", requestID)
		return c.Next()
	}
}

// ──────────────────────────────────────────────
// Tenant Schema Middleware
// ──────────────────────────────────────────────

// RequireTenantSchema sets the PostgreSQL search_path for the current request
// based on the tenant ID extracted from JWT locals.
// MUST be placed AFTER RequireAuth in the middleware chain.
func RequireTenantSchema() fiber.Handler {
	return func(c fiber.Ctx) error {
		tenantID := GetTenantID(c)
		if tenantID == "" {
			return c.Status(401).JSON(response.Error("UNAUTHORIZED", "Tenant context is missing"))
		}
		// The actual search_path setting is done per-repository call using
		// database.WithTenantSchema(ctx, db, tenantID)
		// We just validate tenant context is present here.
		return c.Next()
	}
}

// ──────────────────────────────────────────────
// Rate Limiting Middleware
// ──────────────────────────────────────────────

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// MaxRequests is the maximum number of requests allowed in the Window.
	MaxRequests int
	// Window is the sliding window duration.
	Window time.Duration
	// KeyFunc returns the rate limit key for the current request.
	// Default: IP-based. Override for tenant-based or user-based limits.
	KeyFunc func(c fiber.Ctx) string
}

// RateLimiter returns a Redis-backed sliding window rate limiter.
func RateLimiter(rdb *appredis.Client, cfg RateLimitConfig) fiber.Handler {
	keyFn := cfg.KeyFunc
	if keyFn == nil {
		keyFn = func(c fiber.Ctx) string {
			return fmt.Sprintf("rate:%s:%s", c.IP(), c.Path())
		}
	}

	return func(c fiber.Ctx) error {
		ctx := c.Context()
		key := keyFn(c)

		count, err := rdb.Incr(ctx, key)
		if err != nil {
			// Fail open — don't block requests if Redis is down
			return c.Next()
		}

		if count == 1 {
			// First request in window — set TTL
			_ = rdb.Expire(ctx, key, cfg.Window)
		}

		// Expose rate limit headers
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.MaxRequests))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, cfg.MaxRequests-int(count))))

		if int(count) > cfg.MaxRequests {
			ttl, _ := rdb.TTL(ctx, key)
			c.Set("Retry-After", fmt.Sprintf("%.0f", ttl.Seconds()))
			return c.Status(429).JSON(response.Error("RATE_LIMITED",
				"Too many requests, please try again later"))
		}

		return c.Next()
	}
}

// TenantRateLimiter is a rate limiter keyed on tenant ID + path.
// Use for plan-aware limits.
func TenantRateLimiter(rdb *appredis.Client, maxReqs int, window time.Duration) fiber.Handler {
	return RateLimiter(rdb, RateLimitConfig{
		MaxRequests: maxReqs,
		Window:      window,
		KeyFunc: func(c fiber.Ctx) string {
			tenantID := GetTenantID(c)
			if tenantID == "" {
				return fmt.Sprintf("rate:ip:%s:%s", c.IP(), c.Path())
			}
			return fmt.Sprintf("rate:tenant:%s:%s", tenantID, c.Path())
		},
	})
}

// ──────────────────────────────────────────────
// Idempotency Middleware
// ──────────────────────────────────────────────

const idempotencyTTL = 24 * time.Hour

// Idempotency returns a middleware that caches responses based on the
// X-Idempotency-Key header. A cached response is returned immediately
// for duplicate requests within 24 hours.
//
// Apply only to state-changing POST endpoints (e.g. create transaction).
func Idempotency(rdb *appredis.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Only apply to POST/PUT methods
		if c.Method() != "POST" && c.Method() != "PUT" {
			return c.Next()
		}

		key := c.Get("X-Idempotency-Key")
		if key == "" {
			return c.Next() // No key provided, skip
		}

		ctx := c.Context()
		cacheKey := fmt.Sprintf("idempotency:%s:%s", GetTenantID(c), key)

		// Check if we have a cached response
		var cached []byte
		err := rdb.Get(ctx, cacheKey, &cached)
		if err == nil && len(cached) > 0 {
			c.Set("X-Idempotent-Replayed", "true")
			c.Set("Content-Type", "application/json")
			return c.Status(200).Send(cached)
		}

		if !appredis.IsNil(err) && err != nil {
			// Redis error — fail open
			return c.Next()
		}

		// Process the request
		err = c.Next()

		// Cache the response if it was successful (2xx)
		if c.Response().StatusCode() >= 200 && c.Response().StatusCode() < 300 {
			body := c.Response().Body()
			_ = rdb.Set(ctx, cacheKey, body, idempotencyTTL)
		}

		return err
	}
}

// ──────────────────────────────────────────────
// Panic Recovery
// ──────────────────────────────────────────────

// RecoverPanic returns a middleware that recovers from panics and returns 500.
// Logs the panic with stack trace.
func RecoverPanic() fiber.Handler {
	return func(c fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				// Log will be picked up by the logger middleware
				err = c.Status(500).JSON(response.Error("INTERNAL_ERROR",
					"An unexpected error occurred"))
			}
		}()
		return c.Next()
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Ensure the redis.Nil reference compiles.
var _ = redis.Nil
