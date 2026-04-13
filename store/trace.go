package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
const (
	traceIDKey ctxKey = iota + 1
	spanIDKey
	parentSpanIDKey
	traceFlagsKey
	traceStateKey
	sessionIDKey
)

type TraceContext struct {
	RequestID    string `json:"request_id"`
	TraceID      string `json:"trace_id"`
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
	TraceFlags   string `json:"trace_flags,omitempty"`
	TraceState   string `json:"trace_state,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
}

type TraceParent struct {
	Version      string
	TraceID      string
	ParentSpanID string
	TraceFlags   string
}

const (
	traceparentVersion = "00"
	defaultTraceFlags  = "01"
	maxTraceStateLen   = 512
)

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

func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

func WithSpanID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, spanIDKey, id)
}

func SpanIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(spanIDKey).(string); ok {
		return v
	}
	return ""
}

func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

func WithParentSpanID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, parentSpanIDKey, id)
}

func ParentSpanIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(parentSpanIDKey).(string); ok {
		return v
	}
	return ""
}

func WithTraceFlags(ctx context.Context, flags string) context.Context {
	return context.WithValue(ctx, traceFlagsKey, flags)
}

func TraceFlagsFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceFlagsKey).(string); ok {
		return v
	}
	return ""
}

func WithTraceState(ctx context.Context, state string) context.Context {
	return context.WithValue(ctx, traceStateKey, state)
}

func TraceStateFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceStateKey).(string); ok {
		return v
	}
	return ""
}

func SessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(sessionIDKey).(string); ok {
		return v
	}
	return ""
}

func WithTraceContext(ctx context.Context, trace TraceContext) context.Context {
	if trace.RequestID != "" {
		ctx = WithRequestID(ctx, trace.RequestID)
	}
	if trace.TraceID != "" {
		ctx = WithTraceID(ctx, trace.TraceID)
	}
	if trace.SpanID != "" {
		ctx = WithSpanID(ctx, trace.SpanID)
	}
	if trace.ParentSpanID != "" {
		ctx = WithParentSpanID(ctx, trace.ParentSpanID)
	}
	if trace.TraceFlags != "" {
		ctx = WithTraceFlags(ctx, trace.TraceFlags)
	}
	if trace.TraceState != "" {
		ctx = WithTraceState(ctx, trace.TraceState)
	}
	if trace.SessionID != "" {
		ctx = WithSessionID(ctx, trace.SessionID)
	}
	return ctx
}

func TraceContextFromContext(ctx context.Context) TraceContext {
	return TraceContext{
		RequestID:    RequestIDFromContext(ctx),
		TraceID:      TraceIDFromContext(ctx),
		SpanID:       SpanIDFromContext(ctx),
		ParentSpanID: ParentSpanIDFromContext(ctx),
		TraceFlags:   TraceFlagsFromContext(ctx),
		TraceState:   TraceStateFromContext(ctx),
		SessionID:    SessionIDFromContext(ctx),
	}
}

func NewTraceID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

func NewSpanID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

func ParseTraceparent(value string) (TraceParent, bool) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 4 {
		return TraceParent{}, false
	}

	version := strings.ToLower(parts[0])
	traceID := strings.ToLower(parts[1])
	parentSpanID := strings.ToLower(parts[2])
	traceFlags := strings.ToLower(parts[3])

	if !isValidHex(version, 2) || version == "ff" {
		return TraceParent{}, false
	}
	if !isValidHex(traceID, 32) || isAllZeroHex(traceID) {
		return TraceParent{}, false
	}
	if !isValidHex(parentSpanID, 16) || isAllZeroHex(parentSpanID) {
		return TraceParent{}, false
	}
	if !isValidHex(traceFlags, 2) {
		return TraceParent{}, false
	}

	return TraceParent{
		Version:      version,
		TraceID:      traceID,
		ParentSpanID: parentSpanID,
		TraceFlags:   traceFlags,
	}, true
}

func FormatTraceparent(traceID, spanID, traceFlags string) string {
	flags := strings.ToLower(strings.TrimSpace(traceFlags))
	if !isValidHex(flags, 2) {
		flags = defaultTraceFlags
	}
	return traceparentVersion + "-" + strings.ToLower(traceID) + "-" + strings.ToLower(spanID) + "-" + flags
}

func NormalizeTraceState(value string) string {
	state := strings.TrimSpace(value)
	if state == "" || len(state) > maxTraceStateLen {
		return ""
	}
	for _, char := range state {
		if char < 0x20 || char == 0x7f {
			return ""
		}
	}
	return state
}

func DefaultTraceFlags() string {
	return defaultTraceFlags
}

func IsValidTraceID(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return isValidHex(value, 32) && !isAllZeroHex(value)
}

func isValidHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func isAllZeroHex(value string) bool {
	return strings.Trim(value, "0") == ""
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
	if trace := TraceContextFromContext(ctx); trace != (TraceContext{}) {
		if trace.RequestID != "" {
			e = e.Str("request_id", trace.RequestID)
		}
		if trace.TraceID != "" {
			e = e.Str("trace_id", trace.TraceID)
		}
		if trace.SpanID != "" {
			e = e.Str("span_id", trace.SpanID)
		}
		if trace.SessionID != "" {
			e = e.Str("session_id", trace.SessionID)
		}
		return e
	}
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
