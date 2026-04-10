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
	app.Post("/api/auth/login", loginHandler(state))
	app.Post("/api/auth/refresh", refreshHandler(state))
	app.Post("/api/auth/logout", logoutHandler)
	app.Post("/api/auth/change-password", requireAuth(state.Tokens()), changePasswordHandler(state))
	app.Get("/api/auth/me", requireAuth(state.Tokens()), currentUserHandler(state))
}

// loginHandler godoc
// @Summary 用户登录
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body loginPayload true "登录凭据"
// @Success 200 {object} shared.Response{data=httpserver.loginResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/auth/login [post]
func loginHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
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
	}
}

// refreshHandler godoc
// @Summary 刷新访问令牌
// @Tags Auth
// @Accept json
// @Produce json
// @Param payload body refreshPayload true "刷新令牌"
// @Success 200 {object} shared.Response{data=httpserver.refreshResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/auth/refresh [post]
func refreshHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
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
	}
}

// logoutHandler godoc
// @Summary 用户登出
// @Tags Auth
// @Produce json
// @Success 200 {object} shared.Response{data=httpserver.logoutResponseData}
// @Router /api/auth/logout [post]
func logoutHandler(c fiber.Ctx) error {
	return c.JSON(shared.OK(map[string]any{"logged_out": true}))
}

// changePasswordHandler godoc
// @Summary 修改当前用户密码
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param payload body user.ChangePasswordInput true "修改密码请求"
// @Success 200 {object} shared.Response{data=httpserver.changePasswordResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/auth/change-password [post]
func changePasswordHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
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
	}
}

// currentUserHandler godoc
// @Summary 获取当前登录用户
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} shared.Response{data=httpserver.currentUserResponseData}
// @Failure 401 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/auth/me [get]
func currentUserHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
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
	}
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
