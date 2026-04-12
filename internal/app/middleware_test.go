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

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	pkglogger "github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"
)

func newTestSettingStore(t *testing.T) *store.SettingStore {
	t.Helper()

	cfg := DefaultConfig()
	cfg.Database.Driver = "sqlite"
	cfg.Database.DSN = "file::memory:?cache=shared"

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
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	cfg := DefaultConfig()
	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, cfg, settingStore, &log)
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

func TestSetupMiddlewareChain_RecoversPanics(t *testing.T) {
	settingStore := newTestSettingStore(t)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	cfg := DefaultConfig()
	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, cfg, settingStore, &log)
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
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	cfg := DefaultConfig()
	app := fiber.New()
	log := zerolog.New(io.Discard)
	setupMiddlewareChain(app, cfg, settingStore, &log)
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
