package logger

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

// FiberMiddleware returns a Fiber middleware that logs every HTTP request
// using structured Zap logging with request ID and trace ID propagation.
func FiberMiddleware(log *zap.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// Extract request identifiers
		requestID := c.Get("X-Request-ID")
		traceID := c.Get("X-Trace-ID") // set by tracer middleware (runs before this)

		reqLogger := log.With(
			zap.String("request_id", requestID),
			zap.String("trace_id", traceID),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.String("ip", c.IP()),
			zap.String("user_agent", c.Get("User-Agent")),
		)

		// Inject into context so handlers can use it
		ctx := InjectLogger(c.Context(), reqLogger)
		c.SetContext(ctx)

		// Process request
		err := c.Next()

		// Log after response
		duration := time.Since(start)
		statusCode := c.Response().StatusCode()

		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.Duration("duration", duration),
			zap.String("duration_human", fmt.Sprintf("%.2fms", float64(duration.Microseconds())/1000)),
			zap.Int("response_size", len(c.Response().Body())),
		}

		switch {
		case statusCode >= 500:
			reqLogger.Error("request completed with server error", fields...)
		case statusCode >= 400:
			reqLogger.Warn("request completed with client error", fields...)
		default:
			reqLogger.Info("request completed", fields...)
		}

		return err
	}
}
