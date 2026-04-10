package httpserver

import (
	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/shared"
	"github.com/ysicing/go-template/internal/system"
)

func registerSystemRoutes(app *fiber.App, state *State) {
	handler := func(c fiber.Ctx) error {
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

	app.Get("/api/system/settings", requireAuth(state.Tokens()), requireAdmin, handler)
}
