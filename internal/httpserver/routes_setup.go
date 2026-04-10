package httpserver

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/setup"
	"github.com/ysicing/go-template/internal/shared"
)

func registerSetupRoutes(app *fiber.App, state *State) {
	app.Get("/api/setup/status", setupStatusHandler(state))
	app.Post("/api/setup/install", setupInstallHandler(state))
}

// setupStatusHandler godoc
// @Summary 查询是否需要初始化
// @Tags Setup
// @Produce json
// @Success 200 {object} shared.Response{data=httpserver.setupStatusData}
// @Router /api/setup/status [get]
func setupStatusHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(shared.OK(map[string]any{
			"setup_required": state.SetupRequired(),
		}))
	}
}

// setupInstallHandler godoc
// @Summary 执行首次安装
// @Tags Setup
// @Accept json
// @Produce json
// @Param payload body setup.InstallInput true "安装向导配置"
// @Success 200 {object} shared.Response{data=httpserver.installResultData}
// @Failure 400 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/setup/install [post]
func setupInstallHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
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
	}
}
