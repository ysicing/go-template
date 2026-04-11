package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

type fakeEmailTester struct {
	called bool
	to     string
	err    error
}

func (f *fakeEmailTester) SendTestEmail(_ context.Context, to string) error {
	f.called = true
	f.to = to
	return f.err
}

func TestAdminSettingGet_IncludesSiteTitle(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	h := NewAdminSettingHandler(settings, audit, nil)

	if err := settings.Set(context.Background(), store.SettingSiteTitle, "Acme Console"); err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Get("/api/admin/settings", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if got, _ := body["site_title"].(string); got != "Acme Console" {
		t.Fatalf("expected site_title %q, got %q", "Acme Console", got)
	}
	if _, exists := body["telegram_auto_register_enabled"]; exists {
		t.Fatal("expected telegram_auto_register_enabled to be removed")
	}
	if _, exists := body["wechat_mp_token"]; exists {
		t.Fatal("expected wechat_mp_token to be removed")
	}
}

func TestAdminSettingUpdate_UpdatesSiteTitle(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	h := NewAdminSettingHandler(settings, audit, nil)

	app := fiber.New()
	app.Use(RequestIDMiddleware())
	app.Use(AuditContextMiddleware())
	app.Put("/api/admin/settings", func(c fiber.Ctx) error {
		c.Locals("user_id", "admin-user")
		return h.Update(c)
	})

	req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", strings.NewReader(`{"site_title":"Acme ID"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if got := settings.Get(store.SettingSiteTitle, ""); got != "Acme ID" {
		t.Fatalf("expected site_title %q, got %q", "Acme ID", got)
	}

	assertAdminSettingAuditLog(t, db, "admin-user", model.AuditSettingUpdate, "setting")
}

func TestAdminSettingTestEmail_SendsToProvidedRecipient(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	sender := &fakeEmailTester{}
	h := NewAdminSettingHandler(settings, audit, sender)

	app := fiber.New()
	app.Use(RequestIDMiddleware())
	app.Use(AuditContextMiddleware())
	app.Post("/api/admin/settings/test-email", func(c fiber.Ctx) error {
		c.Locals("user_id", "admin-user")
		return h.TestEmail(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/test-email", strings.NewReader(`{"to":"target@example.com"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !sender.called {
		t.Fatal("expected SendTestEmail to be called")
	}
	if sender.to != "target@example.com" {
		t.Fatalf("expected recipient %q, got %q", "target@example.com", sender.to)
	}

	assertAdminSettingAuditLog(t, db, "admin-user", model.AuditSettingUpdate, "setting_email_test")
}

func assertAdminSettingAuditLog(t *testing.T, db *gorm.DB, userID, action, resource string) {
	t.Helper()

	var auditLog model.AuditLog
	if err := db.WithContext(context.Background()).
		Where("user_id = ? AND action = ? AND resource = ?", userID, action, resource).
		Order("created_at DESC").
		First(&auditLog).Error; err != nil {
		t.Fatalf("expected audit log for action %s: %v", action, err)
	}
}

func TestAdminSettingTestEmail_FallsBackToSMTPFromAddress(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	sender := &fakeEmailTester{}
	h := NewAdminSettingHandler(settings, audit, sender)

	if err := settings.Set(context.Background(), store.SettingSMTPFromAddress, "from@example.com"); err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Post("/api/admin/settings/test-email", func(c fiber.Ctx) error {
		c.Locals("user_id", "admin-user")
		return h.TestEmail(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/test-email", strings.NewReader(`{"to":"  "}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if sender.to != "from@example.com" {
		t.Fatalf("expected fallback recipient %q, got %q", "from@example.com", sender.to)
	}
}

func TestAdminSettingTestEmail_ReturnsBadRequestWhenRecipientMissing(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	sender := &fakeEmailTester{}
	h := NewAdminSettingHandler(settings, audit, sender)

	app := fiber.New()
	app.Post("/api/admin/settings/test-email", h.TestEmail)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/test-email", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if sender.called {
		t.Fatal("expected SendTestEmail not to be called")
	}
}

func TestAdminSettingTestEmail_ReturnsBadRequestWhenAddressInvalid(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	sender := &fakeEmailTester{}
	h := NewAdminSettingHandler(settings, audit, sender)

	app := fiber.New()
	app.Post("/api/admin/settings/test-email", h.TestEmail)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/test-email", strings.NewReader(`{"to":"not-an-email"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if sender.called {
		t.Fatal("expected SendTestEmail not to be called")
	}
}

func TestAdminSettingTestEmail_ReturnsServiceUnavailableWithoutEmailHandler(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	h := NewAdminSettingHandler(settings, audit, nil)

	app := fiber.New()
	app.Post("/api/admin/settings/test-email", h.TestEmail)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/settings/test-email", strings.NewReader(`{"to":"target@example.com"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

func TestAdminSettingGet_IncludesInviteRewardSettings(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	h := NewAdminSettingHandler(settings, audit, nil)

	if err := settings.Set(context.Background(), store.SettingInviteRewardEnabled, "false"); err != nil {
		t.Fatal(err)
	}
	if err := settings.Set(context.Background(), store.SettingInviteRewardMin, "2"); err != nil {
		t.Fatal(err)
	}
	if err := settings.Set(context.Background(), store.SettingInviteRewardMax, "4"); err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Get("/api/admin/settings", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if got, ok := body["invite_reward_enabled"].(bool); !ok || got {
		t.Fatalf("expected invite_reward_enabled false, got %#v", body["invite_reward_enabled"])
	}
	if got, _ := body["invite_reward_min"].(string); got != "2" {
		t.Fatalf("expected invite_reward_min %q, got %q", "2", got)
	}
	if got, _ := body["invite_reward_max"].(string); got != "4" {
		t.Fatalf("expected invite_reward_max %q, got %q", "4", got)
	}
}

func TestAdminSettingUpdate_ValidatesInviteRewardSettings(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	h := NewAdminSettingHandler(settings, audit, nil)

	app := fiber.New()
	app.Put("/api/admin/settings", func(c fiber.Ctx) error {
		c.Locals("user_id", "admin-user")
		return h.Update(c)
	})

	cases := []struct {
		name string
		body string
	}{
		{name: "min out of range", body: `{"invite_reward_min":"0","invite_reward_max":"5"}`},
		{name: "max out of range", body: `{"invite_reward_min":"1","invite_reward_max":"6"}`},
		{name: "min greater than max", body: `{"invite_reward_min":"4","invite_reward_max":"2"}`},
		{name: "non-numeric min", body: `{"invite_reward_min":"abc","invite_reward_max":"4"}`},
		{name: "non-numeric max", body: `{"invite_reward_min":"1","invite_reward_max":"xyz"}`},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != fiber.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func TestAdminSettingUpdate_UpdatesInviteRewardSettings(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	h := NewAdminSettingHandler(settings, audit, nil)

	app := fiber.New()
	app.Put("/api/admin/settings", func(c fiber.Ctx) error {
		c.Locals("user_id", "admin-user")
		return h.Update(c)
	})

	req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", strings.NewReader(`{"invite_reward_enabled":false,"invite_reward_min":"2","invite_reward_max":"5"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if got := settings.GetBool(store.SettingInviteRewardEnabled, true); got {
		t.Fatal("expected invite_reward_enabled to be false")
	}
	if got := settings.Get(store.SettingInviteRewardMin, ""); got != "2" {
		t.Fatalf("expected invite_reward_min %q, got %q", "2", got)
	}
	if got := settings.Get(store.SettingInviteRewardMax, ""); got != "5" {
		t.Fatalf("expected invite_reward_max %q, got %q", "5", got)
	}
}
