package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
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
