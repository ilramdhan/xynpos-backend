package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

// ServiceMetrics holds all Prometheus metrics for a single service.
type ServiceMetrics struct {
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	ActiveRequests       *prometheus.GaugeVec
	BusinessCounter      *prometheus.CounterVec
	BusinessGauge        *prometheus.GaugeVec
}

// New creates and registers all metrics for a service.
// Call once at startup.
func New(serviceName string) *ServiceMetrics {
	namespace := "xynpos"
	subsystem := serviceName

	m := &ServiceMetrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),

		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"method", "path", "status"}),

		ActiveRequests: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "http_active_requests",
			Help:      "Number of currently active HTTP requests",
		}, []string{"method", "path"}),

		BusinessCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "business_events_total",
			Help:      "Total number of business events",
		}, []string{"event", "tenant_id"}),

		BusinessGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "business_metric",
			Help:      "Business metric gauge values",
		}, []string{"name", "tenant_id"}),
	}

	// Register all metrics
	prometheus.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.ActiveRequests,
		m.BusinessCounter,
		m.BusinessGauge,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return m
}

// FiberMiddleware returns a Fiber middleware that records HTTP metrics.
func (m *ServiceMetrics) FiberMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		path := c.Route().Path
		method := c.Method()

		m.ActiveRequests.WithLabelValues(method, path).Inc()
		defer m.ActiveRequests.WithLabelValues(method, path).Dec()

		err := c.Next()

		status := strconv.Itoa(c.Response().StatusCode())
		duration := time.Since(start).Seconds()

		m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(method, path, status).Observe(duration)

		return err
	}
}

// PrometheusHandler returns an HTTP handler for the /metrics endpoint.
// Register this on a separate port (e.g. :9090) to avoid exposing it via API gateway.
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

// IncrementBusinessEvent increments a business event counter.
//
//	metrics.IncrementBusinessEvent("transaction_created", tenantID)
func (m *ServiceMetrics) IncrementBusinessEvent(event, tenantID string) {
	m.BusinessCounter.WithLabelValues(event, tenantID).Inc()
}

// SetBusinessGauge sets a business metric gauge value.
//
//	metrics.SetBusinessGauge("active_sessions", tenantID, 5)
func (m *ServiceMetrics) SetBusinessGauge(name, tenantID string, value float64) {
	m.BusinessGauge.WithLabelValues(name, tenantID).Set(value)
}
