package app

import (
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestManagedOpenAPIRoutesContainCoreRoutes(t *testing.T) {
	routes := managedOpenAPIRoutes()
	want := map[openAPIRouteKey]struct{}{
		{Method: fiber.MethodPost, Path: normalizeOpenAPIPath("/api/auth/login")}:     {},
		{Method: fiber.MethodGet, Path: normalizeOpenAPIPath("/api/users/me")}:        {},
		{Method: fiber.MethodGet, Path: normalizeOpenAPIPath("/api/admin/users/:id")}: {},
	}
	got := make(map[openAPIRouteKey]struct{}, len(routes))
	for _, route := range routes {
		got[openAPIRouteKey{Method: route.Method, Path: normalizeOpenAPIPath(route.Path)}] = struct{}{}
	}
	for key := range want {
		if _, ok := got[key]; !ok {
			t.Fatalf("expected managed route %#v in openapi docs", key)
		}
	}
}
