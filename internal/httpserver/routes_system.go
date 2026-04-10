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
	app.Get("/api/system/settings/mail", requireAuth(state.Tokens()), requireAdmin, mailSettingsHandler(state))
	app.Put("/api/system/settings/mail", requireAuth(state.Tokens()), requireAdmin, updateMailSettingsHandler(state))
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

// mailSettingsHandler godoc
// @Summary 获取邮件设置
// @Tags System
// @Security BearerAuth
// @Produce json
// @Success 200 {object} shared.Response{data=httpserver.mailSettingsResponseData}
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/system/settings/mail [get]
func mailSettingsHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		conn := state.DB()
		if conn == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(shared.Err("SETTINGS_UNAVAILABLE", "settings unavailable"))
		}

		settings, err := system.LoadMailSettings(conn)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(shared.Err("MAIL_SETTINGS_FAILED", "failed to load mail settings"))
		}

		return c.JSON(shared.OK(map[string]any{"mail": sanitizeMailSettings(settings)}))
	}
}

// updateMailSettingsHandler godoc
// @Summary 更新邮件设置
// @Tags System
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param payload body system.MailSettings true "邮件设置"
// @Success 200 {object} shared.Response{data=httpserver.mailSettingsResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/system/settings/mail [put]
func updateMailSettingsHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		conn := state.DB()
		if conn == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(shared.Err("SETTINGS_UNAVAILABLE", "settings unavailable"))
		}

		var payload system.MailSettings
		if err := c.Bind().Body(&payload); err != nil {
			return badRequest(c, "invalid request body")
		}
		if payload.SMTPPort < 0 {
			return badRequest(c, "invalid smtp_port")
		}

		if err := system.SaveMailSettings(conn, payload); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(shared.Err("MAIL_SETTINGS_SAVE_FAILED", "failed to save mail settings"))
		}

		settings, err := system.LoadMailSettings(conn)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(shared.Err("MAIL_SETTINGS_FAILED", "failed to load mail settings"))
		}

		return c.JSON(shared.OK(map[string]any{"mail": sanitizeMailSettings(settings)}))
	}
}

func sanitizeMailSettings(settings system.MailSettings) system.MailSettings {
	settings.Password = ""
	settings.PasswordSet = settings.PasswordSet
	return settings
}
