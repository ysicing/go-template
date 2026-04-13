package store

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	pkglogger "github.com/ysicing/go-template/pkg/logger"
)

func captureTraceLogger(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := pkglogger.L
	pkglogger.L = zerolog.New(&buf).With().Timestamp().Logger().Level(zerolog.InfoLevel)
	return &buf, func() {
		pkglogger.L = prev
	}
}

func TestTraceLogger_SuppressesZeroRowCleanupStatements(t *testing.T) {
	buf, restore := captureTraceLogger(t)
	defer restore()

	l := NewTraceLogger("info").(*traceLogger)
	l.Trace(context.Background(), time.Now().Add(-10*time.Millisecond), func() (string, int64) {
		return `UPDATE "tokens" SET "deleted_at"='2026-04-02 05:14:24.088' WHERE expires_at < '2026-04-02 05:14:24.064' AND "tokens"."deleted_at" IS NULL`, 0
	}, nil)

	if got := buf.String(); got != "" {
		t.Fatalf("expected cleanup statement with 0 rows to be suppressed, got %q", got)
	}
}

func TestTraceLogger_LogsNormalInfoStatements(t *testing.T) {
	buf, restore := captureTraceLogger(t)
	defer restore()

	l := NewTraceLogger("info").(*traceLogger)
	l.Trace(context.Background(), time.Now().Add(-10*time.Millisecond), func() (string, int64) {
		return `SELECT * FROM "settings" WHERE key = 'turnstile_site_key'`, 1
	}, nil)

	if got := buf.String(); !strings.Contains(got, `turnstile_site_key`) {
		t.Fatalf("expected normal info sql to be logged, got %q", got)
	}
}

func TestTraceLogger_LogsTraceFieldsFromContext(t *testing.T) {
	buf, restore := captureTraceLogger(t)
	defer restore()

	ctx := WithTraceContext(context.Background(), TraceContext{
		RequestID: "req-1",
		TraceID:   "trace-1",
		SpanID:    "span-1",
		SessionID: "sess-1",
	})

	l := NewTraceLogger("info").(*traceLogger)
	l.Trace(ctx, time.Now().Add(-10*time.Millisecond), func() (string, int64) {
		return `SELECT * FROM "users"`, 1
	}, nil)

	got := buf.String()
	for _, part := range []string{`"request_id":"req-1"`, `"trace_id":"trace-1"`, `"span_id":"span-1"`, `"session_id":"sess-1"`} {
		if !strings.Contains(got, part) {
			t.Fatalf("expected %s in sql trace log, got %q", part, got)
		}
	}
}

func TestParseTraceparent_Valid(t *testing.T) {
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	parsed, ok := ParseTraceparent(traceparent)
	if !ok {
		t.Fatalf("expected traceparent %q to parse", traceparent)
	}
	if parsed.Version != "00" {
		t.Fatalf("expected version 00, got %q", parsed.Version)
	}
	if parsed.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("unexpected trace id %q", parsed.TraceID)
	}
	if parsed.ParentSpanID != "00f067aa0ba902b7" {
		t.Fatalf("unexpected parent span id %q", parsed.ParentSpanID)
	}
	if parsed.TraceFlags != "01" {
		t.Fatalf("unexpected trace flags %q", parsed.TraceFlags)
	}
}

func TestParseTraceparent_Invalid(t *testing.T) {
	cases := []string{
		"",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
		"00-4bf92f3577b34da6a3ce929d0e0e473g-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-gg",
	}

	for _, traceparent := range cases {
		t.Run(traceparent, func(t *testing.T) {
			if parsed, ok := ParseTraceparent(traceparent); ok {
				t.Fatalf("expected traceparent %q to be rejected, got %#v", traceparent, parsed)
			}
		})
	}
}

func TestFormatTraceparent_NormalizesLowercase(t *testing.T) {
	got := FormatTraceparent("4BF92F3577B34DA6A3CE929D0E0E4736", "00F067AA0BA902B7", "01")
	want := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if got != want {
		t.Fatalf("expected formatted traceparent %q, got %q", want, got)
	}
}
