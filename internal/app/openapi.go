package app

import (
	"slices"
	"strings"

	fiberSwagger "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

type openAPIRoute struct {
	Method       string
	Path         string
	Summary      string
	Tag          string
	RequiresAuth bool
	Permissions  []string
}

type openAPIRouteKey struct {
	Method string
	Path   string
}

type openAPIViewer struct {
	Authenticated bool
	IsAdmin       bool
	Permissions   map[string]struct{}
}

func registerDocsRoutes(app *fiber.App, deps *Deps, buildInfo BuildInfo) {
	optionalJWT := handler.OptionalJWTMiddleware(deps.Config.JWT.Secret, deps.Config.JWT.Issuer)

	app.Get("/openapi.json", optionalJWT, func(c fiber.Ctx) error {
		viewer := resolveOpenAPIViewer(c, deps.UserStore)
		return c.JSON(buildOpenAPIDocument(buildInfo, viewer))
	})

	app.Get("/swagger/*", fiberSwagger.New(fiberSwagger.Config{
		Title:                    "go-template Swagger",
		URL:                      "/openapi.json",
		DeepLinking:              true,
		DisplayRequestDuration:   true,
		DefaultModelsExpandDepth: -1,
	}))
}

func resolveOpenAPIViewer(c fiber.Ctx, users *store.UserStore) openAPIViewer {
	viewer := openAPIViewer{Permissions: map[string]struct{}{}}
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return viewer
	}
	viewer.Authenticated = true
	viewer.IsAdmin, _ = c.Locals("is_admin").(bool)

	if claimsPermissions, ok := c.Locals("permissions").([]string); ok {
		for _, permission := range claimsPermissions {
			permission = strings.TrimSpace(permission)
			if permission != "" {
				viewer.Permissions[permission] = struct{}{}
			}
		}
	}
	if viewer.IsAdmin {
		for _, permission := range model.AllAdminPermissions() {
			viewer.Permissions[permission] = struct{}{}
		}
		return viewer
	}
	if len(viewer.Permissions) > 0 || users == nil {
		return viewer
	}
	user, err := users.GetByID(c.Context(), userID)
	if err != nil {
		return viewer
	}
	for _, permission := range user.PermissionList() {
		viewer.Permissions[permission] = struct{}{}
	}
	return viewer
}

func buildOpenAPIDocument(buildInfo BuildInfo, viewer openAPIViewer) fiber.Map {
	paths := fiber.Map{}
	for _, route := range openAPIRoutes() {
		if !viewerCanAccessRoute(viewer, route) {
			continue
		}
		normalizedPath := normalizeOpenAPIPath(route.Path)
		pathItem, _ := paths[normalizedPath].(fiber.Map)
		if pathItem == nil {
			pathItem = fiber.Map{}
		}
		operation := fiber.Map{
			"tags":        []string{route.Tag},
			"summary":     route.Summary,
			"operationId": openAPIOperationID(route),
			"responses": fiber.Map{
				"200": fiber.Map{"description": "OK"},
			},
		}
		if route.RequiresAuth || len(route.Permissions) > 0 {
			operation["security"] = []fiber.Map{{"bearerAuth": []string{}}}
		}
		if len(route.Permissions) > 0 {
			operation["x-permissions"] = route.Permissions
		}
		pathItem[strings.ToLower(route.Method)] = operation
		paths[normalizedPath] = pathItem
	}

	return fiber.Map{
		"openapi": "3.0.3",
		"info": fiber.Map{
			"title":       "go-template API",
			"version":     emptyFallback(buildInfo.Version, "dev"),
			"description": "Dynamic OpenAPI document filtered by current user permissions.",
		},
		"servers": []fiber.Map{{"url": "/"}},
		"tags":    openAPITags(),
		"components": fiber.Map{
			"securitySchemes": fiber.Map{
				"bearerAuth": fiber.Map{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
		"paths": paths,
	}
}

func viewerCanAccessRoute(viewer openAPIViewer, route openAPIRoute) bool {
	if !route.RequiresAuth && len(route.Permissions) == 0 {
		return true
	}
	if !viewer.Authenticated {
		return false
	}
	if len(route.Permissions) == 0 {
		return true
	}
	if viewer.IsAdmin {
		return true
	}
	for _, permission := range route.Permissions {
		if _, ok := viewer.Permissions[permission]; ok {
			return true
		}
	}
	return false
}

func openAPIOperationID(route openAPIRoute) string {
	replacer := strings.NewReplacer("/", "_", "{", "", "}", "", ":", "", "-", "_")
	return strings.ToLower(route.Method) + replacer.Replace(route.Path)
}

func normalizeOpenAPIPath(path string) string {
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			segments[index] = "{" + strings.TrimPrefix(segment, ":") + "}"
		}
	}
	return strings.Join(segments, "/")
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func openAPITags() []fiber.Map {
	return []fiber.Map{
		{"name": "system"},
		{"name": "auth"},
		{"name": "user"},
		{"name": "session"},
		{"name": "mfa"},
		{"name": "app"},
		{"name": "points"},
		{"name": "admin"},
		{"name": "oauth"},
	}
}

func openAPIRoutes() []openAPIRoute {
	routes := []openAPIRoute{
		{Method: fiber.MethodGet, Path: "/health", Summary: "Health check", Tag: "system"},
		{Method: fiber.MethodGet, Path: "/api/version", Summary: "Build version", Tag: "system"},
		{Method: fiber.MethodGet, Path: "/openapi.json", Summary: "OpenAPI document", Tag: "system"},
		{Method: fiber.MethodGet, Path: "/.well-known/{path...}", Summary: "OIDC discovery metadata", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/authorize", Summary: "OIDC authorize endpoint", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/authorize/callback", Summary: "OIDC authorize callback", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/oauth/userinfo", Summary: "OIDC userinfo endpoint", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/oauth/keys", Summary: "OIDC JWKS endpoint", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/oauth/end_session", Summary: "OIDC end session endpoint", Tag: "oauth"},

		{Method: fiber.MethodGet, Path: "/api/auth/config", Summary: "Read auth config", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/register", Summary: "Register local user", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/login", Summary: "Login with password", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/refresh", Summary: "Refresh tokens", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/logout", Summary: "Logout current session", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/oidc-login", Summary: "Submit OIDC login", Tag: "auth"},
		{Method: fiber.MethodGet, Path: "/api/auth/oidc/consent", Summary: "Read OIDC consent context", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/oidc/consent/approve", Summary: "Approve OIDC consent", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/oidc/consent/deny", Summary: "Deny OIDC consent", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/verify-email", Summary: "Verify email token", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/resend-verification", Summary: "Resend verification email", Tag: "auth", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/auth/mfa/verify", Summary: "Verify MFA code", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/webauthn/begin", Summary: "Begin WebAuthn login", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/webauthn/finish", Summary: "Finish WebAuthn login", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/mfa/webauthn/begin", Summary: "Begin MFA WebAuthn", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/mfa/webauthn/finish", Summary: "Finish MFA WebAuthn", Tag: "auth"},
		{Method: fiber.MethodGet, Path: "/api/auth/github", Summary: "Start GitHub login", Tag: "auth"},
		{Method: fiber.MethodGet, Path: "/api/auth/github/callback", Summary: "Finish GitHub login", Tag: "auth"},
		{Method: fiber.MethodGet, Path: "/api/auth/google", Summary: "Start Google login", Tag: "auth"},
		{Method: fiber.MethodGet, Path: "/api/auth/google/callback", Summary: "Finish Google login", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/social/exchange", Summary: "Exchange social code", Tag: "auth"},
		{Method: fiber.MethodPost, Path: "/api/auth/social/confirm-link", Summary: "Confirm social account link", Tag: "auth"},

		{Method: fiber.MethodPost, Path: "/oauth/token", Summary: "OAuth token endpoint", Tag: "oauth"},
		{Method: fiber.MethodPost, Path: "/oauth/introspect", Summary: "OAuth introspection endpoint", Tag: "oauth"},
		{Method: fiber.MethodPost, Path: "/oauth/revoke", Summary: "OAuth revoke endpoint", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/login/oauth/authorize", Summary: "GitHub compatible authorize", Tag: "oauth"},
		{Method: fiber.MethodPost, Path: "/login/oauth/access_token", Summary: "GitHub compatible token", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/api/v3/user", Summary: "GitHub compatible current user", Tag: "oauth"},
		{Method: fiber.MethodGet, Path: "/api/v3/user/emails", Summary: "GitHub compatible user emails", Tag: "oauth"},

		{Method: fiber.MethodGet, Path: "/api/users/me", Summary: "Get current user", Tag: "user", RequiresAuth: true},
		{Method: fiber.MethodPut, Path: "/api/users/me", Summary: "Update current user", Tag: "user", RequiresAuth: true},
		{Method: fiber.MethodPut, Path: "/api/users/me/password", Summary: "Change password", Tag: "user", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/users/me/set-password", Summary: "Set password", Tag: "user", RequiresAuth: true},
		{Method: fiber.MethodGet, Path: "/api/users/me/login-history", Summary: "Get login history", Tag: "user", RequiresAuth: true},
		{Method: fiber.MethodGet, Path: "/api/users/me/authorized-apps", Summary: "List authorized apps", Tag: "user", RequiresAuth: true},
		{Method: fiber.MethodDelete, Path: "/api/users/me/authorized-apps/:id", Summary: "Revoke authorized app", Tag: "user", RequiresAuth: true},
		{Method: fiber.MethodGet, Path: "/api/users/me/social-accounts", Summary: "List linked social accounts", Tag: "user", RequiresAuth: true},
		{Method: fiber.MethodDelete, Path: "/api/users/me/social-accounts/:id", Summary: "Unlink social account", Tag: "user", RequiresAuth: true},

		{Method: fiber.MethodGet, Path: "/api/sessions/", Summary: "List user sessions", Tag: "session", RequiresAuth: true},
		{Method: fiber.MethodDelete, Path: "/api/sessions/:id", Summary: "Revoke one session", Tag: "session", RequiresAuth: true},
		{Method: fiber.MethodDelete, Path: "/api/sessions/", Summary: "Revoke all sessions", Tag: "session", RequiresAuth: true},

		{Method: fiber.MethodGet, Path: "/api/mfa/status", Summary: "Get MFA status", Tag: "mfa", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/mfa/totp/setup", Summary: "Setup TOTP", Tag: "mfa", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/mfa/totp/enable", Summary: "Enable TOTP", Tag: "mfa", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/mfa/totp/disable", Summary: "Disable TOTP", Tag: "mfa", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/mfa/backup-codes/regenerate", Summary: "Regenerate backup codes", Tag: "mfa", RequiresAuth: true},
		{Method: fiber.MethodGet, Path: "/api/mfa/webauthn/credentials", Summary: "List WebAuthn credentials", Tag: "mfa", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/mfa/webauthn/register/begin", Summary: "Begin WebAuthn registration", Tag: "mfa", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/mfa/webauthn/register/finish", Summary: "Finish WebAuthn registration", Tag: "mfa", RequiresAuth: true},
		{Method: fiber.MethodDelete, Path: "/api/mfa/webauthn/credentials/:id", Summary: "Delete WebAuthn credential", Tag: "mfa", RequiresAuth: true},

		{Method: fiber.MethodGet, Path: "/api/apps/stats", Summary: "Get app stats", Tag: "app", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/apps/", Summary: "Create app", Tag: "app", RequiresAuth: true},
		{Method: fiber.MethodGet, Path: "/api/apps/", Summary: "List apps", Tag: "app", RequiresAuth: true},
		{Method: fiber.MethodGet, Path: "/api/apps/:id", Summary: "Get app", Tag: "app", RequiresAuth: true},
		{Method: fiber.MethodPut, Path: "/api/apps/:id", Summary: "Update app", Tag: "app", RequiresAuth: true},
		{Method: fiber.MethodDelete, Path: "/api/apps/:id", Summary: "Delete app", Tag: "app", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/apps/:id/rotate-secret", Summary: "Rotate app secret", Tag: "app", RequiresAuth: true},

		{Method: fiber.MethodGet, Path: "/api/points/", Summary: "Get point balance", Tag: "points", RequiresAuth: true},
		{Method: fiber.MethodGet, Path: "/api/points/transactions", Summary: "List point transactions", Tag: "points", RequiresAuth: true},
		{Method: fiber.MethodGet, Path: "/api/points/checkin/status", Summary: "Get checkin status", Tag: "points", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/points/checkin", Summary: "Check in for points", Tag: "points", RequiresAuth: true},
		{Method: fiber.MethodPost, Path: "/api/points/spend", Summary: "Spend points", Tag: "points", RequiresAuth: true},

		{Method: fiber.MethodGet, Path: "/api/admin/stats", Summary: "Get admin stats", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminStatsRead}},
		{Method: fiber.MethodGet, Path: "/api/admin/login-history", Summary: "Get login history", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminLoginHistoryRead}},
		{Method: fiber.MethodGet, Path: "/api/admin/audit-logs", Summary: "Get audit logs", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminLoginHistoryRead}},
		{Method: fiber.MethodPost, Path: "/api/admin/users/", Summary: "Create user", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminUsersWrite}},
		{Method: fiber.MethodGet, Path: "/api/admin/users/", Summary: "List users", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminUsersRead}},
		{Method: fiber.MethodGet, Path: "/api/admin/users/:id", Summary: "Get user", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminUsersRead}},
		{Method: fiber.MethodPut, Path: "/api/admin/users/:id", Summary: "Update user", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminUsersWrite}},
		{Method: fiber.MethodDelete, Path: "/api/admin/users/:id", Summary: "Delete user", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminUsersWrite}},
		{Method: fiber.MethodPost, Path: "/api/admin/clients/", Summary: "Create OAuth client", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminClientsWrite}},
		{Method: fiber.MethodGet, Path: "/api/admin/clients/", Summary: "List OAuth clients", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminClientsRead}},
		{Method: fiber.MethodGet, Path: "/api/admin/clients/:id", Summary: "Get OAuth client", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminClientsRead}},
		{Method: fiber.MethodPut, Path: "/api/admin/clients/:id", Summary: "Update OAuth client", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminClientsWrite}},
		{Method: fiber.MethodDelete, Path: "/api/admin/clients/:id", Summary: "Delete OAuth client", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminClientsWrite}},
		{Method: fiber.MethodGet, Path: "/api/admin/providers/", Summary: "List social providers", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminProvidersRead}},
		{Method: fiber.MethodGet, Path: "/api/admin/providers/:id", Summary: "Get social provider", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminProvidersRead}},
		{Method: fiber.MethodPost, Path: "/api/admin/providers/", Summary: "Create social provider", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminProvidersWrite}},
		{Method: fiber.MethodPut, Path: "/api/admin/providers/:id", Summary: "Update social provider", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminProvidersWrite}},
		{Method: fiber.MethodDelete, Path: "/api/admin/providers/:id", Summary: "Delete social provider", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminProvidersWrite}},
		{Method: fiber.MethodGet, Path: "/api/admin/settings/", Summary: "Get settings", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminSettingsRead}},
		{Method: fiber.MethodPut, Path: "/api/admin/settings/", Summary: "Update settings", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminSettingsWrite}},
		{Method: fiber.MethodPost, Path: "/api/admin/settings/test-email", Summary: "Send test email", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminSettingsWrite}},
		{Method: fiber.MethodPost, Path: "/api/admin/points/adjust", Summary: "Adjust user points", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminPointsWrite}},
		{Method: fiber.MethodGet, Path: "/api/admin/points/transactions", Summary: "List point transactions", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminPointsRead}},
		{Method: fiber.MethodGet, Path: "/api/admin/points/leaderboard", Summary: "Get points leaderboard", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminPointsRead}},
		{Method: fiber.MethodGet, Path: "/api/admin/points/:user_id", Summary: "Get user points", Tag: "admin", RequiresAuth: true, Permissions: []string{model.PermissionAdminPointsRead}},
	}
	slices.SortFunc(routes, func(left, right openAPIRoute) int {
		if left.Path == right.Path {
			return strings.Compare(left.Method, right.Method)
		}
		return strings.Compare(left.Path, right.Path)
	})
	return routes
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
		keys = append(keys, openAPIRouteKey{
			Method: strings.ToUpper(route.Method),
			Path:   normalizeOpenAPIPath(route.Path),
		})
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
		key := openAPIRouteKey{
			Method: strings.ToUpper(route.Method),
			Path:   normalizeOpenAPIPath(route.Path),
		}
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
	if _, ok := oidcMountedPathSet()[key.Path]; ok {
		return true
	}
	return false
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
	case path == "/health":
		return true
	case path == "/api/version":
		return true
	case path == "/openapi.json":
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
	case path == "/authorize":
		return true
	case path == "/authorize/callback":
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
