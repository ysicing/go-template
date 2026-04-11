package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/pkg/logger"

	gormlogger "gorm.io/gorm/logger"
)

type ctxKey int

const requestIDKey ctxKey = iota

// WithRequestID returns a child context carrying the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext extracts the request ID from ctx, or "".
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// traceLogger is a GORM logger that uses zerolog.
type traceLogger struct {
	level                gormlogger.LogLevel
	slowThreshold        time.Duration
	ignoreRecordNotFound bool
}

// NewTraceLogger creates a GORM logger that outputs via zerolog with request ID.
func NewTraceLogger(level string) gormlogger.Interface {
	return &traceLogger{
		level:                parseLogLevel(level),
		slowThreshold:        200 * time.Millisecond,
		ignoreRecordNotFound: true,
	}
}

// parseLogLevel maps the unified log.level to GORM log level.
// "debug"/"info" → show all SQL, "warn" → slow queries + errors, "error" → errors only.
func parseLogLevel(s string) gormlogger.LogLevel {
	switch s {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "warn":
		return gormlogger.Warn
	default:
		return gormlogger.Info
	}
}

func (l *traceLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	nl := *l
	nl.level = level
	return &nl
}

func (l *traceLogger) logEvent(ctx context.Context, lvl zerolog.Level) *zerolog.Event {
	e := logger.L.WithLevel(lvl)
	if id := RequestIDFromContext(ctx); id != "" {
		e = e.Str("request_id", id)
	}
	return e
}

func (l *traceLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Info {
		l.logEvent(ctx, zerolog.InfoLevel).Msgf(msg, data...)
	}
}

func (l *traceLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Warn {
		l.logEvent(ctx, zerolog.WarnLevel).Msgf(msg, data...)
	}
}

func (l *traceLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Error {
		l.logEvent(ctx, zerolog.ErrorLevel).Msgf(msg, data...)
	}
}

func (l *traceLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	switch {
	case err != nil && (!l.ignoreRecordNotFound || !isRecordNotFound(err)):
		l.logEvent(ctx, zerolog.ErrorLevel).
			Float64("elapsed_ms", float64(elapsed.Nanoseconds())/1e6).
			Int64("rows", rows).
			Err(err).
			Msg(sql)
	case elapsed > l.slowThreshold && l.slowThreshold > 0:
		l.logEvent(ctx, zerolog.WarnLevel).
			Str("slow", "true").
			Dur("threshold", l.slowThreshold).
			Float64("elapsed_ms", float64(elapsed.Nanoseconds())/1e6).
			Int64("rows", rows).
			Msg(sql)
	case l.level >= gormlogger.Info:
		if shouldSuppressTraceLog(sql, rows, elapsed, l.slowThreshold) {
			return
		}
		l.logEvent(ctx, zerolog.InfoLevel).
			Float64("elapsed_ms", float64(elapsed.Nanoseconds())/1e6).
			Int64("rows", rows).
			Msg(sql)
	}
}

func shouldSuppressTraceLog(sql string, rows int64, elapsed, slowThreshold time.Duration) bool {
	if rows != 0 {
		return false
	}
	if slowThreshold > 0 && elapsed > slowThreshold {
		return false
	}

	normalized := strings.ToLower(strings.TrimSpace(sql))
	return strings.HasPrefix(normalized, `update "api_refresh_tokens" set "deleted_at"`) ||
		strings.HasPrefix(normalized, `update "auth_requests" set "deleted_at"`) ||
		strings.HasPrefix(normalized, `update "tokens" set "deleted_at"`)
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
