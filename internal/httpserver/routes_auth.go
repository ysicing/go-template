package httpserver

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/shared"
	"github.com/ysicing/go-template/internal/user"
	"gorm.io/gorm"
)

type loginPayload struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type refreshPayload struct {
	RefreshToken string `json:"refresh_token"`
}

func registerAuthRoutes(app *fiber.App, state *State) {
	app.Post("/api/auth/login", func(c fiber.Ctx) error {
		service, err := requireAuthService(c, state)
		if err != nil {
			return err
		}

		var payload loginPayload
		if err := c.Bind().Body(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(shared.Err("BAD_REQUEST", "invalid request body"))
		}

		account, pair, err := service.Login(payload.Identifier, payload.Password)
		if err != nil {
			return writeAuthServiceError(c, err, "LOGIN_FAILED", "failed to login")
		}

		return c.JSON(shared.OK(map[string]any{
			"user":  account,
			"token": pair,
		}))
	})

	app.Post("/api/auth/refresh", func(c fiber.Ctx) error {
		service, err := requireAuthService(c, state)
		if err != nil {
			return err
		}

		var payload refreshPayload
		if err := c.Bind().Body(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(shared.Err("BAD_REQUEST", "invalid request body"))
		}

		pair, err := service.Refresh(payload.RefreshToken)
		if err != nil {
			return writeAuthServiceError(c, err, "REFRESH_FAILED", "failed to refresh token")
		}

		return c.JSON(shared.OK(map[string]any{"token": pair}))
	})

	app.Post("/api/auth/logout", func(c fiber.Ctx) error {
		return c.JSON(shared.OK(map[string]any{"logged_out": true}))
	})

	app.Post("/api/auth/change-password", requireAuth(state.Tokens()), func(c fiber.Ctx) error {
		service, err := requireUserService(c, state)
		if err != nil {
			return err
		}

		userID, _ := c.Locals(localUserID).(uint)
		var payload user.ChangePasswordInput
		if err := c.Bind().Body(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(shared.Err("BAD_REQUEST", "invalid request body"))
		}
		if err := service.ChangePassword(userID, payload); err != nil {
			return writeUserServiceError(c, err, "CHANGE_PASSWORD_FAILED", "failed to change password")
		}
		return c.JSON(shared.OK(map[string]any{"changed": true}))
	})

	app.Get("/api/auth/me", requireAuth(state.Tokens()), func(c fiber.Ctx) error {
		service, err := requireAuthService(c, state)
		if err != nil {
			return err
		}

		userID, _ := c.Locals(localUserID).(uint)
		account, err := service.CurrentUser(userID)
		if err != nil {
			return writeCurrentUserError(c, err)
		}

		return c.JSON(shared.OK(map[string]any{"user": account}))
	})
}

func requireAuthService(c fiber.Ctx, state *State) (*auth.Service, error) {
	service := state.Auth()
	if service == nil {
		return nil, c.Status(fiber.StatusServiceUnavailable).JSON(shared.Err("AUTH_UNAVAILABLE", "auth unavailable"))
	}
	return service, nil
}

func writeAuthServiceError(c fiber.Ctx, err error, internalCode string, internalMessage string) error {
	status, code, message := mapAuthServiceError(err, internalCode, internalMessage)
	return c.Status(status).JSON(shared.Err(code, message))
}

func mapAuthServiceError(err error, internalCode string, internalMessage string) (int, string, string) {
	if errors.Is(err, auth.ErrInvalidCredentials) {
		return fiber.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid credentials"
	}
	if errors.Is(err, auth.ErrInvalidRefreshToken) {
		return fiber.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "invalid refresh token"
	}
	return fiber.StatusInternalServerError, internalCode, internalMessage
}

func writeCurrentUserError(c fiber.Ctx, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusUnauthorized).JSON(shared.Err("UNAUTHORIZED", "unauthorized"))
	}
	return c.Status(fiber.StatusInternalServerError).JSON(shared.Err("AUTH_ME_FAILED", "failed to load current user"))
}
