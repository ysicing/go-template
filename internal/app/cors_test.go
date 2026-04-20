package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func setupSettingStoreTest(t *testing.T) *store.SettingStore {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}

	return store.NewSettingStore(db, store.NewMemoryCache())
}

func TestBuildCORSMiddleware_TogglesCredentialsWhenWildcardChanges(t *testing.T) {
	settings := setupSettingStoreTest(t)
	if err := settings.Set(context.Background(), store.SettingCORSOrigins, "https://app.example.com"); err != nil {
		t.Fatal(err)
	}

	log := zerolog.New(io.Discard)
	fiberApp := fiber.New()
	fiberApp.Use(buildCORSMiddleware(settings, &log))
	fiberApp.Get("/ping", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req1.Header.Set("Origin", "https://app.example.com")
	resp1, err := fiberApp.Test(req1)
	if err != nil {
		t.Fatal(err)
	}
	if resp1.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}
	if got := resp1.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header true for specific origins, got %q", got)
	}

	if err := settings.Set(context.Background(), store.SettingCORSOrigins, "*"); err != nil {
		t.Fatal(err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req2.Header.Set("Origin", "https://evil.example.com")
	resp2, err := fiberApp.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	if got := resp2.Header.Get("Access-Control-Allow-Credentials"); got == "true" {
		t.Fatalf("expected credentials header disabled for wildcard origins, got %q", got)
	}
}

func TestBuildCORSMiddleware_DeniesCrossOriginWhenOriginsUnset(t *testing.T) {
	settings := setupSettingStoreTest(t)

	log := zerolog.New(io.Discard)
	fiberApp := fiber.New()
	fiberApp.Use(buildCORSMiddleware(settings, &log))
	fiberApp.Get("/ping", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://app.example.com")
	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS allow origin header when origins unset, got %q", got)
	}
}
