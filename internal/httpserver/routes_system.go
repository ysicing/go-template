package httpserver

import (
	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/buildinfo"
	"github.com/ysicing/go-template/internal/shared"
	"github.com/ysicing/go-template/internal/system"
)

func registerSystemRoutes(app *fiber.App, state *State) {
	app.Get("/api/system/version", systemVersionHandler)
	app.Get("/api/system/settings", requireAuth(state.Tokens()), requireAdmin, systemSettingsHandler(state))
}

// systemVersionHandler godoc
// @Summary 获取构建版本信息
// @Tags System
// @Produce json
// @Success 200 {object} shared.Response{data=httpserver.versionResponseData}
// @Router /api/system/version [get]
func systemVersionHandler(c fiber.Ctx) error {
	return c.JSON(shared.OK(buildinfo.Current()))
}

// systemSettingsHandler godoc
// @Summary 获取系统设置
// @Tags System
// @Security BearerAuth
// @Produce json
// @Success 200 {object} shared.Response{data=httpserver.settingsResponseData}
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/system/settings [get]
func systemSettingsHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		conn := state.DB()
		if conn == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(shared.Err("SETTINGS_UNAVAILABLE", "settings unavailable"))
		}

		var settings []system.Setting
		if err := conn.Order("id asc").Find(&settings).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(shared.Err("SETTINGS_FAILED", err.Error()))
		}
		return c.JSON(shared.OK(map[string]any{"items": settings}))
	}
}
