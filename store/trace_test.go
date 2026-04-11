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
