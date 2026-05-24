package tracer

import (
	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

// FiberMiddleware returns a Fiber middleware that:
//   - Extracts incoming trace context from W3C headers
//   - Starts a new root span for each HTTP request
//   - Injects trace/span IDs into response headers for client correlation
func FiberMiddleware(serviceName string) fiber.Handler {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(c fiber.Ctx) error {
		// Extract propagated trace context from incoming request headers
		ctx := propagator.Extract(c.Context(), propagation.HeaderCarrier(c.GetReqHeaders()))

		// Start span
		spanName := c.Method() + " " + c.Route().Path
		ctx, span := tracer.Start(ctx, spanName)
		defer span.End()

		// Set span attributes
		span.SetAttributes(
			attribute.String("http.method", c.Method()),
			attribute.String("http.url", c.OriginalURL()),
			attribute.String("http.scheme", c.Protocol()),
			attribute.String("http.host", c.Hostname()),
			attribute.String("net.peer.ip", c.IP()),
		)

		// Expose trace/span IDs in response headers (useful for debugging)
		sc := span.SpanContext()
		if sc.HasTraceID() {
			c.Set("X-Trace-ID", sc.TraceID().String())
			c.Set("X-Span-ID", sc.SpanID().String())
		}

		// Replace context in Fiber's local storage
		c.SetContext(ctx)

		// Process the request
		err := c.Next()

		// Record status code on span
		status := c.Response().StatusCode()
		span.SetAttributes(attribute.Int("http.status_code", status))
		if status >= 500 {
			span.SetStatus(2, "server error") // codes.Error = 2
		}

		return err
	}
}

// gRPC helpers for context propagation are in grpc_interceptors.go
