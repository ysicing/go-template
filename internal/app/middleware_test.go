package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pkglogger "github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	fibermemory "github.com/gofiber/storage/memory/v2"
	"github.com/rs/zerolog"
)

func newTestSettingStore(t *testing.T) *store.SettingStore {
	t.Helper()

	cfg := DefaultConfig()
	cfg.Database.Driver = "sqlite"
	cfg.Database.DSN = testSQLiteDSN(t)

	log := zerolog.New(io.Discard)
	db, cache := initDBAndCache(context.Background(), cfg, &log)
	t.Cleanup(func() {
		_ = cache.Close()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return store.NewSettingStore(db, cache)
}

func TestSecurityHeadersMiddleware_AllowsInlineScript(t *testing.T) {
	settingStore := newTestSettingStore(t)
	app := fiber.New()
	app.Use(securityHeadersMiddleware(settingStore))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}

	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("expected inline script allowance in CSP, got %q", csp)
	}
}

func TestSecurityHeadersMiddleware_AllowsInlineScriptWithTurnstile(t *testing.T) {
	settingStore := newTestSettingStore(t)
	if err := settingStore.Set(context.Background(), store.SettingTurnstileSiteKey, "site-key"); err != nil {
		t.Fatalf("seed turnstile key: %v", err)
	}

	app := fiber.New()
	app.Use(securityHeadersMiddleware(settingStore))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}

	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self' 'unsafe-inline' https://challenges.cloudflare.com") {
		t.Fatalf("expected turnstile and inline script allowance in CSP, got %q", csp)
	}
}

func TestRequestLogMiddleware_SkipsKubeProbeUA(t *testing.T) {
	var buf bytes.Buffer
	prev := pkglogger.L
	pkglogger.L = zerolog.New(&buf)
	t.Cleanup(func() { pkglogger.L = prev })

	app := fiber.New()
	app.Use(requestLogMiddleware())
	app.Get("/health", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("User-Agent", "kube-probe/1.34")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if strings.Contains(buf.String(), "\"message\":\"request\"") || strings.Contains(buf.String(), "request") {
		t.Fatalf("expected kube-probe request log to be skipped, got %q", buf.String())
	}
}

func TestSetupMiddlewareChain_PropagatesTraceContext(t *testing.T) {
	settingStore := newTestSettingStore(t)

	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, settingStore, fibermemory.New(), &log)
	app.Get("/trace", func(c fiber.Ctx) error {
		return c.JSON(store.TraceContextFromContext(c.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Header.Get("X-Request-ID"); got != "req-123" {
		t.Fatalf("expected request id header to be preserved, got %q", got)
	}
	if got := resp.Header.Get("X-Trace-ID"); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("expected response trace id header, got %q", got)
	}

	var trace store.TraceContext
	if err := json.NewDecoder(resp.Body).Decode(&trace); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if trace.RequestID != "req-123" {
		t.Fatalf("expected request id in context, got %q", trace.RequestID)
	}
	if trace.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("expected trace id in context, got %q", trace.TraceID)
	}
	if trace.SpanID == "" {
		t.Fatal("expected span id in context")
	}
}

func TestSetupMiddlewareChain_GeneratesIndependentTraceAndSpanIDs(t *testing.T) {
	settingStore := newTestSettingStore(t)

	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, settingStore, fibermemory.New(), &log)
	app.Get("/trace", func(c fiber.Ctx) error {
		return c.JSON(store.TraceContextFromContext(c.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	req.Header.Set("X-Request-ID", "req-123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	var trace store.TraceContext
	if err := json.NewDecoder(resp.Body).Decode(&trace); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if trace.RequestID != "req-123" {
		t.Fatalf("expected request id in context, got %q", trace.RequestID)
	}
	if trace.TraceID == "" || trace.TraceID == trace.RequestID {
		t.Fatalf("expected independent trace id, got %q", trace.TraceID)
	}
	if len(trace.TraceID) != 32 {
		t.Fatalf("expected 32-char trace id, got %q", trace.TraceID)
	}
	if trace.SpanID == "" || trace.SpanID == trace.TraceID || trace.SpanID == trace.RequestID {
		t.Fatalf("expected independent span id, got %q", trace.SpanID)
	}
	if len(trace.SpanID) != 16 {
		t.Fatalf("expected 16-char span id, got %q", trace.SpanID)
	}
}

func TestSetupMiddlewareChain_PropagatesW3CTraceHeaders(t *testing.T) {
	settingStore := newTestSettingStore(t)

	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, settingStore, fibermemory.New(), &log)
	app.Get("/trace", func(c fiber.Ctx) error {
		return c.JSON(store.TraceContextFromContext(c.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("Traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("Tracestate", "vendor=value")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	var trace store.TraceContext
	if err := json.NewDecoder(resp.Body).Decode(&trace); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if trace.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("expected propagated trace id, got %q", trace.TraceID)
	}
	if trace.ParentSpanID != "00f067aa0ba902b7" {
		t.Fatalf("expected parent span id, got %q", trace.ParentSpanID)
	}
	if trace.TraceFlags != "01" {
		t.Fatalf("expected trace flags 01, got %q", trace.TraceFlags)
	}
	if trace.TraceState != "vendor=value" {
		t.Fatalf("expected tracestate to be preserved, got %q", trace.TraceState)
	}
	if trace.SpanID == "" || trace.SpanID == trace.ParentSpanID {
		t.Fatalf("expected a fresh span id, got %q", trace.SpanID)
	}

	if got := resp.Header.Get("Traceparent"); got != "00-4bf92f3577b34da6a3ce929d0e0e4736-"+trace.SpanID+"-01" {
		t.Fatalf("expected response traceparent to use current span, got %q", got)
	}
	if got := resp.Header.Get("Tracestate"); got != "vendor=value" {
		t.Fatalf("expected response tracestate, got %q", got)
	}
}

func TestSetupMiddlewareChain_InvalidTraceparentStartsNewTrace(t *testing.T) {
	settingStore := newTestSettingStore(t)

	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, settingStore, fibermemory.New(), &log)
	app.Get("/trace", func(c fiber.Ctx) error {
		return c.JSON(store.TraceContextFromContext(c.Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("Traceparent", "00-00000000000000000000000000000000-00f067aa0ba902b7-01")
	req.Header.Set("Tracestate", "vendor=value")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	var trace store.TraceContext
	if err := json.NewDecoder(resp.Body).Decode(&trace); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if trace.TraceID == "" || trace.TraceID == trace.RequestID {
		t.Fatalf("expected a new independent trace id, got %q", trace.TraceID)
	}
	if len(trace.TraceID) != 32 {
		t.Fatalf("expected 32-char trace id, got %q", trace.TraceID)
	}
	if trace.ParentSpanID != "" {
		t.Fatalf("expected no parent span id for invalid traceparent, got %q", trace.ParentSpanID)
	}
	if got := resp.Header.Get("Traceparent"); got != "00-"+trace.TraceID+"-"+trace.SpanID+"-01" {
		t.Fatalf("expected regenerated response traceparent, got %q", got)
	}
	if got := resp.Header.Get("Tracestate"); got != "" {
		t.Fatalf("expected tracestate to be dropped for invalid traceparent, got %q", got)
	}
}

func TestSetupMiddlewareChain_RecoversPanics(t *testing.T) {
	settingStore := newTestSettingStore(t)

	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, settingStore, fibermemory.New(), &log)
	app.Get("/panic", func(c fiber.Ctx) error {
		panic("boom")
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/panic", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected 500 for panic recovery, got %d", resp.StatusCode)
	}
}

func TestSetupMiddlewareChain_SetsSessionCookieOnlyForBrowserRoutes(t *testing.T) {
	settingStore := newTestSettingStore(t)

	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, settingStore, fibermemory.New(), &log)
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Get("/api/ping", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	if cookie := resp.Header.Get("Set-Cookie"); !strings.Contains(cookie, "session_id=") {
		t.Fatalf("expected browser route to issue session cookie, got %q", cookie)
	}

	apiResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/ping", nil))
	if err != nil {
		t.Fatal(err)
	}
	if cookie := apiResp.Header.Get("Set-Cookie"); strings.Contains(cookie, "session_id=") {
		t.Fatalf("expected api route to skip session cookie, got %q", cookie)
	}
}

func TestCookieCSRFMiddleware_RejectsCookieBackedUnsafeRequestWithoutOrigin(t *testing.T) {
	settingStore := newTestSettingStore(t)
	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, settingStore, fibermemory.New(), &log)
	app.Post("/api/auth/refresh", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.Header.Set("Cookie", "refresh_token=test-refresh")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCookieCSRFMiddleware_AllowsSameOriginCookieRequest(t *testing.T) {
	settingStore := newTestSettingStore(t)
	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, settingStore, fibermemory.New(), &log)
	app.Post("/api/auth/refresh", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.Host = "app.example.com"
	req.Header.Set("Cookie", "refresh_token=test-refresh")
	req.Header.Set("Origin", "http://app.example.com")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCookieCSRFMiddleware_AllowsBearerRequestWithoutCookie(t *testing.T) {
	settingStore := newTestSettingStore(t)
	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, settingStore, fibermemory.New(), &log)
	app.Post("/api/admin/settings", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(http.MethodPost, "/api/admin/settings", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
