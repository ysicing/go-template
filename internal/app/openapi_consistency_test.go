package app

import (
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

func TestFindUndocumentedRoutes_FindsMissingRoute(t *testing.T) {
	app := fiber.New()
	app.Get("/api/openapi-missing", func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	missing := findUndocumentedRoutes(app)
	if len(missing) != 1 {
		t.Fatalf("expected 1 undocumented route, got %d", len(missing))
	}
	if missing[0].Method != fiber.MethodGet || missing[0].Path != "/api/openapi-missing" {
		t.Fatalf("unexpected undocumented route: %#v", missing[0])
	}
}

func TestOpenAPIRoutesCoverRegisteredRoutes(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	log := zerolog.New(io.Discard)
	provider := initOIDCProvider(deps.Config, deps, &log)
	deps.OIDCHandler = provider

	registerSystemRoutes(app, BuildInfo{Version: "test"})
	registerDocsRoutes(app, deps, BuildInfo{Version: "test"})
	SetupRoutes(app, deps)
	mountOIDCHandler(app, provider)

	missing := findUndocumentedRoutes(app)
	if len(missing) != 0 {
		t.Fatalf("expected no undocumented routes, got %#v", missing)
	}

	stale := findStaleDocumentedRoutes(app)
	if len(stale) != 0 {
		t.Fatalf("expected no stale documented routes, got %#v", stale)
	}
}

func TestFindStaleDocumentedRoutes_FindsRemovedRoute(t *testing.T) {
	app := fiber.New()
	app.Get("/health", func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	stale := findStaleDocumentedRoutes(app)
	if len(stale) == 0 {
		t.Fatal("expected stale documented routes")
	}
}
