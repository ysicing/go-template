package app

import (
	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

func registerAdminModule(admin fiber.Router, h *builtHandlers, users *store.UserStore, cache store.Cache, jwtMW, tokenVersionMW fiber.Handler) {
	admin.Get("/stats", jwtMW, tokenVersionMW, handler.RequirePermission(users, cache, model.PermissionAdminStatsRead), h.admin.GetStats)
	admin.Get("/login-history", jwtMW, tokenVersionMW, handler.RequirePermission(users, cache, model.PermissionAdminLoginHistoryRead), h.admin.GetLoginHistory)
	admin.Get("/audit-logs", jwtMW, tokenVersionMW, handler.RequirePermission(users, cache, model.PermissionAdminLoginHistoryRead), h.admin.GetAuditLogs)

	usersGroup := admin.Group("/users", jwtMW, tokenVersionMW)
	usersGroup.Post("/", handler.RequirePermission(users, cache, model.PermissionAdminUsersWrite), h.admin.CreateUser)
	usersGroup.Get("/", handler.RequirePermission(users, cache, model.PermissionAdminUsersRead), h.admin.ListUsers)
	usersGroup.Get("/:id", handler.RequirePermission(users, cache, model.PermissionAdminUsersRead), h.admin.GetUser)
	usersGroup.Put("/:id", handler.RequirePermission(users, cache, model.PermissionAdminUsersWrite), h.admin.UpdateUser)
	usersGroup.Delete("/:id", handler.RequirePermission(users, cache, model.PermissionAdminUsersWrite), h.admin.DeleteUser)

	clients := admin.Group("/clients", jwtMW, tokenVersionMW)
	clients.Post("/", handler.RequirePermission(users, cache, model.PermissionAdminClientsWrite), h.oauthClient.Create)
	clients.Get("/", handler.RequirePermission(users, cache, model.PermissionAdminClientsRead), h.oauthClient.List)
	clients.Get("/:id", handler.RequirePermission(users, cache, model.PermissionAdminClientsRead), h.oauthClient.Get)
	clients.Put("/:id", handler.RequirePermission(users, cache, model.PermissionAdminClientsWrite), h.oauthClient.Update)
	clients.Delete("/:id", handler.RequirePermission(users, cache, model.PermissionAdminClientsWrite), h.oauthClient.Delete)

	providers := admin.Group("/providers", jwtMW, tokenVersionMW)
	providers.Get("/", handler.RequirePermission(users, cache, model.PermissionAdminProvidersRead), h.adminProv.List)
	providers.Get("/:id", handler.RequirePermission(users, cache, model.PermissionAdminProvidersRead), h.adminProv.Get)
	providers.Post("/", handler.RequirePermission(users, cache, model.PermissionAdminProvidersWrite), h.adminProv.Create)
	providers.Put("/:id", handler.RequirePermission(users, cache, model.PermissionAdminProvidersWrite), h.adminProv.Update)
	providers.Delete("/:id", handler.RequirePermission(users, cache, model.PermissionAdminProvidersWrite), h.adminProv.Delete)

	settings := admin.Group("/settings", jwtMW, tokenVersionMW)
	settings.Get("/", handler.RequirePermission(users, cache, model.PermissionAdminSettingsRead), h.adminSett.Get)
	settings.Put("/", handler.RequirePermission(users, cache, model.PermissionAdminSettingsWrite), h.adminSett.Update)
	settings.Post("/test-email", handler.RequirePermission(users, cache, model.PermissionAdminSettingsWrite), h.adminSett.TestEmail)

	adminPoints := admin.Group("/points", jwtMW, tokenVersionMW)
	adminPoints.Post("/adjust", handler.RequirePermission(users, cache, model.PermissionAdminPointsWrite), h.adminPoints.AdjustPoints)
	adminPoints.Get("/transactions", handler.RequirePermission(users, cache, model.PermissionAdminPointsRead), h.adminPoints.GetAllTransactions)
	adminPoints.Get("/leaderboard", handler.RequirePermission(users, cache, model.PermissionAdminPointsRead), h.adminPoints.GetLeaderboard)
	adminPoints.Get("/:user_id", handler.RequirePermission(users, cache, model.PermissionAdminPointsRead), h.adminPoints.GetUserPoints)
}
