package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ysicing/go-template/model"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

func testRouteDeps(t *testing.T) *Deps {
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

	deps := initDeps(context.Background(), db, cache, cfg, &log)
	deps.Config = cfg
	return deps
}

func TestTestRouteDeps_IsolatesDatabasePerTestHelper(t *testing.T) {
	first := testRouteDeps(t)
	second := testRouteDeps(t)

	user := &model.User{
		Username:   "isolation-user",
		Email:      "isolation@example.com",
		Provider:   "local",
		ProviderID: "isolation-user",
		InviteCode: "INV-ISOLATION",
	}
	if err := user.SetPassword("Password123!abcd"); err != nil {
		t.Fatalf("set user password: %v", err)
	}
	if err := first.UserStore.Create(context.Background(), user); err != nil {
		t.Fatalf("create user in first deps: %v", err)
	}

	got, err := second.UserStore.GetByUsername(context.Background(), user.Username)
	if err == nil && got != nil {
		t.Fatalf("expected second deps database to be isolated, but found user %q", user.Username)
	}
}

func TestSetupRoutesRegistersTemplateEndpoints(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)

	SetupRoutes(app, deps)

	for _, tc := range []struct {
		method  string
		path    string
		want404 bool
	}{
		{method: http.MethodPost, path: "/api/auth/login"},
		{method: http.MethodGet, path: "/api/apps", want404: true},
		{method: http.MethodGet, path: "/api/points"},
		{method: http.MethodGet, path: "/api/admin/users"},
		{method: http.MethodGet, path: "/api/admin/settings"},
		{method: http.MethodGet, path: "/api/organizations/test", want404: true},
		{method: http.MethodGet, path: "/api/workspace-plans", want404: true},
		{method: http.MethodGet, path: "/api/webhooks", want404: true},
		{method: http.MethodGet, path: "/api/monitoring/alerts", want404: true},
		{method: http.MethodGet, path: "/api/quotes/random", want404: true},
		{method: http.MethodGet, path: "/api/admin/quotes", want404: true},
		{method: http.MethodGet, path: "/api/admin/clients", want404: true},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if tc.want404 && resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected route %s %s to be removed, got %d", tc.method, tc.path, resp.StatusCode)
		}
		if !tc.want404 && resp.StatusCode == http.StatusNotFound {
			t.Fatalf("expected route %s %s to be registered, got 404", tc.method, tc.path)
		}
	}
}

func TestAuthConfigReturnsTemplateFlags(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	SetupRoutes(app, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("get auth config: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		RegisterEnabled       bool   `json:"register_enabled"`
		TurnstileSiteKey      string `json:"turnstile_site_key"`
		EmailVerificationMode bool   `json:"email_verification_enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode auth config: %v", err)
	}
	if !body.RegisterEnabled {
		t.Fatal("expected registration to stay enabled by default")
	}
	if body.TurnstileSiteKey != "" {
		t.Fatalf("expected empty turnstile site key by default, got %q", body.TurnstileSiteKey)
	}
}
