package app

import (
	"bytes"
	"context"
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
