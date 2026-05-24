package logger

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey string

const loggerKey contextKey = "logger"

// New creates a new Zap logger based on environment.
// In production: JSON format. In development: colored console.
func New(level, env string) *zap.Logger {
	var zapCfg zap.Config

	if env == "production" || env == "staging" {
		zapCfg = zap.NewProductionConfig()
		zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Parse log level
	switch strings.ToLower(level) {
	case "debug":
		zapCfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "warn":
		zapCfg.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		zapCfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		zapCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	logger, err := zapCfg.Build(zap.AddCallerSkip(0))
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	return logger
}

// InjectLogger injects a logger into the context.
func InjectLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves the logger from context.
// Falls back to a no-op logger if not found (safe for tests).
func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerKey).(*zap.Logger); ok && logger != nil {
		return logger
	}
	return zap.NewNop()
}

// WithTraceID returns a logger with trace_id and span_id fields injected.
// Should be called in middleware after trace context is extracted.
func WithTraceID(logger *zap.Logger, traceID, spanID string) *zap.Logger {
	return logger.With(
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	)
}

// WithTenant returns a logger with tenant context.
func WithTenant(logger *zap.Logger, tenantID string) *zap.Logger {
	return logger.With(zap.String("tenant_id", tenantID))
}

// WithRequest returns a logger enriched with request metadata.
func WithRequest(logger *zap.Logger, requestID, method, path string) *zap.Logger {
	return logger.With(
		zap.String("request_id", requestID),
		zap.String("method", method),
		zap.String("path", path),
	)
}

// ──────────────────────────────────────────────
// PII Masking Helpers
// ──────────────────────────────────────────────

// MaskEmail masks an email address: john.doe@example.com → j***@example.com
func MaskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || len(parts[0]) == 0 {
		return "***"
	}
	masked := string(parts[0][0]) + strings.Repeat("*", len(parts[0])-1)
	return masked + "@" + parts[1]
}

// MaskPhone masks a phone number: 081234567890 → 0812****7890
func MaskPhone(phone string) string {
	if len(phone) < 8 {
		return "****"
	}
	return phone[:4] + strings.Repeat("*", len(phone)-8) + phone[len(phone)-4:]
}

// MaskString masks any generic string, keeping only the first char.
func MaskString(s string) string {
	if s == "" {
		return ""
	}
	return string(s[0]) + strings.Repeat("*", len(s)-1)
}
