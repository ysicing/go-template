package httpserver

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/shared"
	"github.com/ysicing/go-template/internal/setup"
)

func registerSetupRoutes(app *fiber.App, state *State) {
	app.Get("/api/setup/status", func(c fiber.Ctx) error {
		return c.JSON(shared.OK(map[string]any{
			"setup_required": state.SetupRequired(),
		}))
	})

	app.Post("/api/setup/install", func(c fiber.Ctx) error {
		if !state.SetupRequired() {
			return c.Status(fiber.StatusForbidden).JSON(shared.Err("SETUP_LOCKED", "setup already completed"))
		}

		var payload setup.InstallInput
		if err := c.Bind().Body(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(shared.Err("BAD_REQUEST", "invalid request body"))
		}

		service := state.Setup()
		if service == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(shared.Err("SETUP_UNAVAILABLE", "setup unavailable"))
		}
		if err := service.Install(context.Background(), payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(shared.Err("INSTALL_FAILED", err.Error()))
		}
		if err := state.ReloadFromConfig(service.ConfigPath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(shared.Err("INSTALL_RELOAD_FAILED", err.Error()))
		}

		return c.JSON(shared.OK(map[string]any{"installed": true}))
	})
}
