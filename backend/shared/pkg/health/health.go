package health

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	appredis "github.com/extendedsynaptic/xynpos/shared/pkg/redis"
)

// Status represents the health of the service.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Response is the JSON body returned by /health and /ready.
type Response struct {
	Status    Status            `json:"status"`
	Service   string            `json:"service"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]Check  `json:"checks,omitempty"`
}

// Check holds the result of a single dependency health check.
type Check struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Handler provides /health and /ready Fiber handlers.
type Handler struct {
	serviceName string
	db          *gorm.DB
	redis       *appredis.Client
}

// New creates a new health Handler.
func New(serviceName string, db *gorm.DB, redis *appredis.Client) *Handler {
	return &Handler{serviceName: serviceName, db: db, redis: redis}
}

// Liveness handles GET /health.
// Returns 200 if the service process is alive (no dependency checks).
func (h *Handler) Liveness(c fiber.Ctx) error {
	return c.JSON(Response{
		Status:    StatusHealthy,
		Service:   h.serviceName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Readiness handles GET /ready.
// Returns 200 only if all critical dependencies are reachable.
func (h *Handler) Readiness(c fiber.Ctx) error {
	checks := make(map[string]Check)
	overallStatus := StatusHealthy

	// Check PostgreSQL
	if h.db != nil {
		start := time.Now()
		sqlDB, err := h.db.DB()
		var dbCheck Check
		if err != nil || sqlDB.PingContext(context.Background()) != nil {
			dbCheck = Check{Status: StatusUnhealthy, Message: "database unreachable"}
			overallStatus = StatusUnhealthy
		} else {
			dbCheck = Check{Status: StatusHealthy, Latency: time.Since(start).String()}
		}
		checks["database"] = dbCheck
	}

	// Check Redis
	if h.redis != nil {
		start := time.Now()
		var redisCheck Check
		if err := h.redis.Ping(context.Background()); err != nil {
			redisCheck = Check{Status: StatusUnhealthy, Message: "redis unreachable"}
			if overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		} else {
			redisCheck = Check{Status: StatusHealthy, Latency: time.Since(start).String()}
		}
		checks["redis"] = redisCheck
	}

	resp := Response{
		Status:    overallStatus,
		Service:   h.serviceName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	}

	httpStatus := 200
	if overallStatus == StatusUnhealthy {
		httpStatus = 503
	}

	body, _ := json.Marshal(resp)
	c.Set("Content-Type", "application/json")
	return c.Status(httpStatus).Send(body)
}

// Register registers /health and /ready routes on a Fiber app.
func (h *Handler) Register(app *fiber.App) {
	app.Get("/health", h.Liveness)
	app.Get("/ready", h.Readiness)
}
