package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP request metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "id_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "id_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Authentication metrics
	authAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "id_auth_attempts_total",
			Help: "Total number of authentication attempts",
		},
		[]string{"type", "status"}, // type: login, register, mfa; status: success, failure
	)

	// Email metrics
	emailSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "id_email_sent_total",
			Help: "Total number of emails sent",
		},
		[]string{"type", "status"}, // type: verification, resend; status: success, failure
	)

	emailRetries = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "id_email_retries_total",
			Help: "Total number of email send retries",
		},
	)

	// Audit log metrics
	auditQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "id_audit_queue_size",
			Help: "Current size of the audit log queue",
		},
	)

	auditDropped = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "id_audit_dropped_total",
			Help: "Total number of dropped audit log entries",
		},
	)

	// Cache metrics
	cacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "id_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	cacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "id_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	// Database metrics
	dbQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "id_db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"operation"}, // operation: select, insert, update, delete
	)

	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "id_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
)

// PrometheusMiddleware records HTTP request metrics.
func PrometheusMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		path := c.Path()
		if !shouldRecordHTTPMetrics(path) {
			return err
		}

		// Record metrics
		duration := time.Since(start).Seconds()
		status := c.Response().StatusCode()
		method := strings.Clone(c.Method())
		metricPath := metricRoutePath(c, path)

		httpRequestsTotal.WithLabelValues(method, metricPath, strconv.Itoa(status)).Inc()
		httpRequestDuration.WithLabelValues(method, metricPath).Observe(duration)

		return err
	}
}

func shouldRecordHTTPMetrics(path string) bool {
	// Do not track static assets and other non-API traffic.
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	// Avoid scraping endpoint self-observation noise.
	if path == "/api/admin/metrics" {
		return false
	}
	return true
}

func metricRoutePath(c fiber.Ctx, fallbackPath string) string {
	if route := c.Route(); route != nil {
		if route.Path != "" {
			return strings.Clone(route.Path)
		}
	}
	return strings.Clone(fallbackPath)
}

// RecordAuthAttempt records an authentication attempt.
func RecordAuthAttempt(authType, status string) {
	authAttemptsTotal.WithLabelValues(authType, status).Inc()
}

// RecordEmailSent records an email send attempt.
func RecordEmailSent(emailType, status string) {
	emailSentTotal.WithLabelValues(emailType, status).Inc()
}

// RecordEmailRetry records an email retry.
func RecordEmailRetry() {
	emailRetries.Inc()
}

// UpdateAuditQueueSize updates the audit queue size metric.
func UpdateAuditQueueSize(size int) {
	auditQueueSize.Set(float64(size))
}

// RecordAuditDropped records a dropped audit log entry.
func RecordAuditDropped() {
	auditDropped.Inc()
}

// RecordCacheHit records a cache hit.
func RecordCacheHit() {
	cacheHits.Inc()
}

// RecordCacheMiss records a cache miss.
func RecordCacheMiss() {
	cacheMisses.Inc()
}

// RecordDBQuery records a database query.
func RecordDBQuery(operation string, duration time.Duration) {
	dbQueriesTotal.WithLabelValues(operation).Inc()
	dbQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}
