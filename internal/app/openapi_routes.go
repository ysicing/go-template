package app

import (
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func openAPIRoutes() []openAPIRoute {
	routes := append(staticOpenAPIRoutes(), managedOpenAPIRoutes()...)
	slices.SortFunc(routes, func(left, right openAPIRoute) int {
		if left.Path == right.Path {
			return strings.Compare(left.Method, right.Method)
		}
		return strings.Compare(left.Path, right.Path)
	})
	return routes
}

func staticOpenAPIRoutes() []openAPIRoute {
	return []openAPIRoute{
		{Method: fiber.MethodGet, Path: "/health", Summary: "Health check", Tag: "system"},
		{Method: fiber.MethodGet, Path: "/api/version", Summary: "Build version", Tag: "system"},
		{Method: fiber.MethodGet, Path: "/openapi.json", Summary: "OpenAPI document", Tag: "system"},
		{Method: fiber.MethodGet, Path: "/.well-known/{path...}", Summary: "OIDC discovery metadata", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/authorize", Summary: "OIDC authorize endpoint", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/authorize/callback", Summary: "OIDC authorize callback", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/oauth/userinfo", Summary: "OIDC userinfo endpoint", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/oauth/keys", Summary: "OIDC JWKS endpoint", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/oauth/end_session", Summary: "OIDC end session endpoint", Tag: "oauth"},
	}
}

func findUndocumentedRoutes(app *fiber.App) []openAPIRouteKey {
	documented := documentedRouteKeySet()
	registered := registeredDocumentableRouteKeys(app)
	missing := make([]openAPIRouteKey, 0)
	for _, route := range registered {
		if _, ok := documented[route]; ok {
			continue
		}
		missing = append(missing, route)
	}
	return missing
}

func findStaleDocumentedRoutes(app *fiber.App) []openAPIRouteKey {
	documented := documentedRouteKeys()
	registered := registeredDocumentableRouteKeySet(app)
	stale := make([]openAPIRouteKey, 0)
	for _, route := range documented {
		if _, ok := registered[route]; ok {
			continue
		}
		stale = append(stale, route)
	}
	return stale
}

func documentedRouteKeys() []openAPIRouteKey {
	keys := make([]openAPIRouteKey, 0, len(openAPIRoutes()))
	for _, route := range openAPIRoutes() {
		keys = append(keys, openAPIRouteKey{Method: strings.ToUpper(route.Method), Path: normalizeOpenAPIPath(route.Path)})
	}
	slices.SortFunc(keys, compareOpenAPIRouteKey)
	return keys
}

func documentedRouteKeySet() map[openAPIRouteKey]struct{} {
	keys := documentedRouteKeys()
	keySet := make(map[openAPIRouteKey]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	return keySet
}

func registeredDocumentableRouteKeys(app *fiber.App) []openAPIRouteKey {
	routes := make([]openAPIRouteKey, 0)
	seen := make(map[openAPIRouteKey]struct{})
	documented := documentedRouteKeySet()
	for _, route := range app.GetRoutes(true) {
		if !shouldDocumentFiberRoute(route) {
			continue
		}
		key := openAPIRouteKey{Method: strings.ToUpper(route.Method), Path: normalizeOpenAPIPath(route.Path)}
		if shouldIgnoreRegisteredRouteKey(key, documented) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		routes = append(routes, key)
	}
	slices.SortFunc(routes, compareOpenAPIRouteKey)
	return routes
}

func shouldIgnoreRegisteredRouteKey(key openAPIRouteKey, documented map[openAPIRouteKey]struct{}) bool {
	if _, ok := documented[key]; ok {
		return false
	}
	_, ok := oidcMountedPathSet()[key.Path]
	return ok
}

func registeredDocumentableRouteKeySet(app *fiber.App) map[openAPIRouteKey]struct{} {
	keys := registeredDocumentableRouteKeys(app)
	keySet := make(map[openAPIRouteKey]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	return keySet
}

func shouldDocumentFiberRoute(route fiber.Route) bool {
	method := strings.ToUpper(route.Method)
	switch method {
	case fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodDelete, fiber.MethodPatch:
	default:
		return false
	}

	path := route.Path
	switch {
	case path == "/health", path == "/api/version", path == "/openapi.json":
		return true
	case strings.HasPrefix(path, "/swagger"):
		return false
	case strings.HasPrefix(path, "/api/"):
		return true
	case strings.HasPrefix(path, "/oauth/"):
		return true
	case strings.HasPrefix(path, "/login/oauth/"):
		return true
	case strings.HasPrefix(path, "/.well-known/"):
		return true
	case path == "/authorize", path == "/authorize/callback":
		return true
	default:
		return false
	}
}

func oidcMountedPathSet() map[string]struct{} {
	return map[string]struct{}{
		normalizeOpenAPIPath("/.well-known/{path...}"): {},
		normalizeOpenAPIPath("/authorize"):             {},
		normalizeOpenAPIPath("/authorize/callback"):    {},
		normalizeOpenAPIPath("/oauth/userinfo"):        {},
		normalizeOpenAPIPath("/oauth/keys"):            {},
		normalizeOpenAPIPath("/oauth/introspect"):      {},
		normalizeOpenAPIPath("/oauth/revoke"):          {},
		normalizeOpenAPIPath("/oauth/end_session"):     {},
	}
}

func compareOpenAPIRouteKey(left, right openAPIRouteKey) int {
	if left.Path == right.Path {
		return strings.Compare(left.Method, right.Method)
	}
	return strings.Compare(left.Path, right.Path)
}
