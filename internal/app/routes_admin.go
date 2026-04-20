package app

import (
	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"

	"github.com/gofiber/fiber/v3"
)

func adminRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	routes := make([]managedRouteSpec, 0)
	routes = append(routes, adminCoreRouteSpecs(rt)...)
	routes = append(routes, adminUserRouteSpecs(rt)...)
	routes = append(routes, adminClientRouteSpecs(rt)...)
	routes = append(routes, adminProviderRouteSpecs(rt)...)
	routes = append(routes, adminSettingRouteSpecs(rt)...)
	routes = append(routes, adminPointRouteSpecs(rt)...)
	return routes
}

func adminCoreRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		adminRoute(fiber.MethodGet, "/api/admin/stats", "Get admin stats", model.PermissionAdminStatsRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.admin.GetStats }),
		adminRoute(fiber.MethodGet, "/api/admin/login-history", "Get login history", model.PermissionAdminLoginHistoryRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.admin.GetLoginHistory }),
		adminRoute(fiber.MethodGet, "/api/admin/audit-logs", "Get audit logs", model.PermissionAdminLoginHistoryRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.admin.GetAuditLogs }),
	}
}

func adminUserRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		adminRoute(fiber.MethodPost, "/api/admin/users", "Create user", model.PermissionAdminUsersWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.admin.CreateUser }),
		adminRoute(fiber.MethodGet, "/api/admin/users", "List users", model.PermissionAdminUsersRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.admin.ListUsers }),
		adminRoute(fiber.MethodGet, "/api/admin/users/:id", "Get user", model.PermissionAdminUsersRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.admin.GetUser }),
		adminRoute(fiber.MethodPut, "/api/admin/users/:id", "Update user", model.PermissionAdminUsersWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.admin.UpdateUser }),
		adminRoute(fiber.MethodDelete, "/api/admin/users/:id", "Delete user", model.PermissionAdminUsersWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.admin.DeleteUser }),
	}
}

func adminClientRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		adminRoute(fiber.MethodPost, "/api/admin/clients", "Create OAuth client", model.PermissionAdminClientsWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.oauthClient.Create }),
		adminRoute(fiber.MethodGet, "/api/admin/clients", "List OAuth clients", model.PermissionAdminClientsRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.oauthClient.List }),
		adminRoute(fiber.MethodGet, "/api/admin/clients/:id", "Get OAuth client", model.PermissionAdminClientsRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.oauthClient.Get }),
		adminRoute(fiber.MethodPut, "/api/admin/clients/:id", "Update OAuth client", model.PermissionAdminClientsWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.oauthClient.Update }),
		adminRoute(fiber.MethodDelete, "/api/admin/clients/:id", "Delete OAuth client", model.PermissionAdminClientsWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.oauthClient.Delete }),
	}
}

func adminProviderRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		adminRoute(fiber.MethodGet, "/api/admin/providers", "List social providers", model.PermissionAdminProvidersRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminProv.List }),
		adminRoute(fiber.MethodGet, "/api/admin/providers/:id", "Get social provider", model.PermissionAdminProvidersRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminProv.Get }),
		adminRoute(fiber.MethodPost, "/api/admin/providers", "Create social provider", model.PermissionAdminProvidersWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminProv.Create }),
		adminRoute(fiber.MethodPut, "/api/admin/providers/:id", "Update social provider", model.PermissionAdminProvidersWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminProv.Update }),
		adminRoute(fiber.MethodDelete, "/api/admin/providers/:id", "Delete social provider", model.PermissionAdminProvidersWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminProv.Delete }),
	}
}

func adminSettingRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		adminRoute(fiber.MethodGet, "/api/admin/settings", "Get settings", model.PermissionAdminSettingsRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminSett.Get }),
		adminRoute(fiber.MethodPut, "/api/admin/settings", "Update settings", model.PermissionAdminSettingsWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminSett.Update }),
		adminRoute(fiber.MethodPost, "/api/admin/settings/test-email", "Send test email", model.PermissionAdminSettingsWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminSett.TestEmail }),
	}
}

func adminPointRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		adminRoute(fiber.MethodPost, "/api/admin/points/adjust", "Adjust user points", model.PermissionAdminPointsWrite, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminPoints.AdjustPoints }),
		adminRoute(fiber.MethodGet, "/api/admin/points/transactions", "List point transactions", model.PermissionAdminPointsRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminPoints.GetAllTransactions }),
		adminRoute(fiber.MethodGet, "/api/admin/points/leaderboard", "Get points leaderboard", model.PermissionAdminPointsRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminPoints.GetLeaderboard }),
		adminRoute(fiber.MethodGet, "/api/admin/points/:user_id", "Get user points", model.PermissionAdminPointsRead, func(rt managedRouteRuntime) fiber.Handler { return rt.handlers.adminPoints.GetUserPoints }),
	}
}

func adminRoute(method, path, summary, permission string, target func(managedRouteRuntime) fiber.Handler) managedRouteSpec {
	return managedRouteSpec{
		Doc: openAPIRoute{Method: method, Path: path, Summary: summary, Tag: "admin", RequiresAuth: true, Permissions: []string{permission}},
		Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, handler.RequirePermission(rt.deps.UserStore, rt.deps.Cache, permission), target(rt)}
		},
	}
}
