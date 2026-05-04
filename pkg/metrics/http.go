package metrics

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

// PrometheusMiddleware 记录 HTTP 请求指标。
func PrometheusMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()

		// 先执行请求链，确保能记录最终状态码。
		err := c.Next()

		path := c.Path()
		if !shouldRecordHTTPMetrics(path) {
			return err
		}

		// 仅记录 API 请求，避免静态资源和抓取端点污染指标。
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
	// 不记录静态资源和其他非 API 流量。
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	// 避免 metrics 抓取端点自观测噪声。
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

// RecordAuthAttempt 记录认证尝试结果。
func RecordAuthAttempt(authType, status string) {
	authAttemptsTotal.WithLabelValues(authType, status).Inc()
}

// RecordEmailSent 记录邮件发送结果。
func RecordEmailSent(emailType, status string) {
	emailSentTotal.WithLabelValues(emailType, status).Inc()
}

// RecordEmailRetry 记录邮件重试次数。
func RecordEmailRetry() {
	emailRetries.Inc()
}

// UpdateAuditQueueSize 更新审计队列长度指标。
func UpdateAuditQueueSize(size int) {
	auditQueueSize.Set(float64(size))
}

// RecordAuditDropped 记录被丢弃的审计日志数量。
func RecordAuditDropped() {
	auditDropped.Inc()
}

// RecordCacheHit 记录缓存命中。
func RecordCacheHit() {
	cacheHits.Inc()
}

// RecordCacheMiss 记录缓存未命中。
func RecordCacheMiss() {
	cacheMisses.Inc()
}

// RecordDBQuery 记录数据库查询耗时。
func RecordDBQuery(operation string, duration time.Duration) {
	dbQueriesTotal.WithLabelValues(operation).Inc()
	dbQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}
