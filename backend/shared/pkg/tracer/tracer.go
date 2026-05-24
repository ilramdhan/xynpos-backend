package tracer

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds tracer configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	JaegerURL      string // e.g. "localhost:4317" (OTLP gRPC)
	Enabled        bool
}

// Init initializes the OpenTelemetry tracer and returns a shutdown function.
// Call defer shutdown() in main().
func Init(ctx context.Context, cfg Config) (func(context.Context), error) {
	if !cfg.Enabled {
		// Install a no-op provider so all trace calls are safe to make
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return func(context.Context) {}, nil
	}

	// Connect to Jaeger via OTLP gRPC
	conn, err := grpc.NewClient(cfg.JaegerURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("tracer: dial jaeger %s: %w", cfg.JaegerURL, err)
	}

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("tracer: create otlp exporter: %w", err)
	}

	// Resource attributes shown in Jaeger UI
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracer: create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Use ParentBased in production
	)

	// Register as global tracer provider
	otel.SetTracerProvider(tp)

	// W3C Trace Context propagation (X-B3 etc.)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdown := func(ctx context.Context) {
		_ = tp.Shutdown(ctx)
		_ = conn.Close()
	}

	return shutdown, nil
}

// Tracer returns a named tracer for the given service.
func Tracer(serviceName string) trace.Tracer {
	return otel.Tracer(serviceName)
}

// StartSpan starts a new span and returns the context with the span.
// Always call defer span.End().
//
//	ctx, span := tracer.StartSpan(ctx, "UserUsecase.Create")
//	defer span.End()
func StartSpan(ctx context.Context, operationName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer("xynpos").Start(ctx, operationName, opts...)
}

// RecordError records an error on the span and marks it as errored.
func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// SetAttributes sets key-value attributes on the span.
func SetAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

// SpanFromContext extracts the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// TraceID returns the trace ID string from the current context span.
func TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID returns the span ID string from the current context span.
func SpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasSpanID() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}
