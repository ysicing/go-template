package app

import (
	"strings"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	fiberSwagger "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
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
			"parameters":  openAPITraceHeaderParameters(),
			"responses": fiber.Map{
				"200": fiber.Map{
					"description": "OK",
					"headers":     openAPITraceResponseHeaders(),
				},
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
			"description": "Dynamic OpenAPI document filtered by current user permissions. Successful responses include `X-Request-ID`, `X-Trace-ID`, and `Traceparent` headers for request correlation, where `X-Trace-ID` is the compatibility-friendly trace identifier header.",
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

func openAPITraceHeaderParameters() []fiber.Map {
	return []fiber.Map{
		{
			"name":        "X-Request-ID",
			"in":          "header",
			"required":    false,
			"description": "Optional caller-supplied request correlation ID. When omitted, the server generates one.",
			"schema": fiber.Map{
				"type": "string",
			},
		},
		{
			"name":        "Traceparent",
			"in":          "header",
			"required":    false,
			"description": "Optional W3C trace context header used to continue an upstream distributed trace.",
			"schema": fiber.Map{
				"type":    "string",
				"example": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			},
		},
		{
			"name":        "Tracestate",
			"in":          "header",
			"required":    false,
			"description": "Optional W3C tracestate header propagated when a valid Traceparent is supplied.",
			"schema": fiber.Map{
				"type":    "string",
				"example": "vendor=value",
			},
		},
	}
}

func openAPITraceResponseHeaders() fiber.Map {
	return fiber.Map{
		"X-Request-ID": fiber.Map{
			"description": "Request correlation ID for this response.",
			"schema": fiber.Map{
				"type": "string",
			},
		},
		"X-Trace-ID": fiber.Map{
			"description": "Compatibility trace identifier propagated across HTTP logs, audit logs, and database query logs.",
			"schema": fiber.Map{
				"type": "string",
			},
		},
		"Traceparent": fiber.Map{
			"description": "W3C trace context response header containing the current trace id and server span id.",
			"schema": fiber.Map{
				"type": "string",
			},
		},
		"Tracestate": fiber.Map{
			"description": "W3C tracestate response header echoed when the request continues a valid upstream trace.",
			"schema": fiber.Map{
				"type": "string",
			},
		},
	}
}

func viewerCanAccessRoute(viewer openAPIViewer, route openAPIRoute) bool {
	if !route.RequiresAuth && len(route.Permissions) == 0 {
		return true
	}
	if !viewer.Authenticated {
		return false
	}
	if len(route.Permissions) == 0 || viewer.IsAdmin {
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
