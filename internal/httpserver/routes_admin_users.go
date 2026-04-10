package httpserver

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/shared"
	"github.com/ysicing/go-template/internal/user"
	"gorm.io/gorm"
)

func registerAdminUserRoutes(app *fiber.App, state *State) {
	app.Get("/api/admin/users", requireAuth(state.Tokens()), requireAdmin, listUsersHandler(state))
	app.Get("/api/admin/users/:id", requireAuth(state.Tokens()), requireAdmin, getUserHandler(state))
	app.Post("/api/admin/users", requireAuth(state.Tokens()), requireAdmin, createUserHandler(state))
	app.Put("/api/admin/users/:id", requireAuth(state.Tokens()), requireAdmin, updateUserHandler(state))
	app.Post("/api/admin/users/:id/enable", requireAuth(state.Tokens()), requireAdmin, enableUserHandler(state))
	app.Post("/api/admin/users/:id/disable", requireAuth(state.Tokens()), requireAdmin, disableUserHandler(state))
	app.Delete("/api/admin/users/:id", requireAuth(state.Tokens()), requireAdmin, deleteUserHandler(state))
}

// listUsersHandler godoc
// @Summary 查询用户列表
// @Tags Admin Users
// @Security BearerAuth
// @Produce json
// @Param keyword query string false "用户名或邮箱关键字"
// @Param role query string false "角色过滤" Enums(user,admin)
// @Param status query string false "状态过滤" Enums(active,disabled)
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} shared.Response{data=user.ListUsersResult}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/admin/users [get]
func listUsersHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		service, err := requireUserService(c, state)
		if err != nil {
			return err
		}

		page, err := parseOptionalInt(c.Query("page"))
		if err != nil {
			return badRequest(c, "invalid page")
		}
		pageSize, err := parseOptionalInt(c.Query("page_size"))
		if err != nil {
			return badRequest(c, "invalid page_size")
		}

		result, err := service.ListUsers(user.ListUsersQuery{
			Keyword:  c.Query("keyword"),
			Role:     c.Query("role"),
			Status:   c.Query("status"),
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			return writeUserServiceError(c, err, "LIST_USERS_FAILED", "failed to list users")
		}
		return c.JSON(shared.OK(result))
	}
}

// getUserHandler godoc
// @Summary 查看单个用户
// @Tags Admin Users
// @Security BearerAuth
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} shared.Response{data=httpserver.singleUserResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 404 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/admin/users/{id} [get]
func getUserHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		service, err := requireUserService(c, state)
		if err != nil {
			return err
		}

		userID, err := parseUserID(c)
		if err != nil {
			return err
		}

		account, err := service.GetUser(userID)
		if err != nil {
			return writeUserServiceError(c, err, "GET_USER_FAILED", "failed to get user")
		}
		return c.JSON(shared.OK(map[string]any{"user": account}))
	}
}

// createUserHandler godoc
// @Summary 创建用户
// @Tags Admin Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param payload body user.CreateUserInput true "新用户信息"
// @Success 200 {object} shared.Response{data=httpserver.singleUserResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 409 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/admin/users [post]
func createUserHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		service, err := requireUserService(c, state)
		if err != nil {
			return err
		}

		var payload user.CreateUserInput
		if err := c.Bind().Body(&payload); err != nil {
			return badRequest(c, "invalid request body")
		}

		account, err := service.CreateUser(payload)
		if err != nil {
			return writeUserServiceError(c, err, "CREATE_USER_FAILED", "failed to create user")
		}
		return c.JSON(shared.OK(map[string]any{"user": account}))
	}
}

// updateUserHandler godoc
// @Summary 更新用户
// @Tags Admin Users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "用户 ID"
// @Param payload body user.UpdateUserInput true "用户更新信息"
// @Success 200 {object} shared.Response{data=httpserver.singleUserResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 404 {object} shared.Response
// @Failure 409 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/admin/users/{id} [put]
func updateUserHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		service, err := requireUserService(c, state)
		if err != nil {
			return err
		}

		userID, err := parseUserID(c)
		if err != nil {
			return err
		}

		var payload user.UpdateUserInput
		if err := c.Bind().Body(&payload); err != nil {
			return badRequest(c, "invalid request body")
		}

		account, err := service.UpdateUser(userID, payload)
		if err != nil {
			return writeUserServiceError(c, err, "UPDATE_USER_FAILED", "failed to update user")
		}
		return c.JSON(shared.OK(map[string]any{"user": account}))
	}
}

// enableUserHandler godoc
// @Summary 启用用户
// @Tags Admin Users
// @Security BearerAuth
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} shared.Response{data=httpserver.enableUserResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 404 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/admin/users/{id}/enable [post]
func enableUserHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		service, err := requireUserService(c, state)
		if err != nil {
			return err
		}

		userID, err := parseUserID(c)
		if err != nil {
			return err
		}

		if err := service.EnableUser(userID); err != nil {
			return writeUserServiceError(c, err, "ENABLE_USER_FAILED", "failed to enable user")
		}
		return c.JSON(shared.OK(map[string]any{"enabled": true}))
	}
}

// disableUserHandler godoc
// @Summary 停用用户
// @Tags Admin Users
// @Security BearerAuth
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} shared.Response{data=httpserver.disableUserResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 404 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/admin/users/{id}/disable [post]
func disableUserHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		service, err := requireUserService(c, state)
		if err != nil {
			return err
		}

		userID, err := parseUserID(c)
		if err != nil {
			return err
		}

		actorID, _ := c.Locals(localUserID).(uint)
		if err := service.DisableUser(actorID, userID); err != nil {
			return writeUserServiceError(c, err, "DISABLE_USER_FAILED", "failed to disable user")
		}
		return c.JSON(shared.OK(map[string]any{"disabled": true}))
	}
}

// deleteUserHandler godoc
// @Summary 删除用户
// @Tags Admin Users
// @Security BearerAuth
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} shared.Response{data=httpserver.deleteUserResponseData}
// @Failure 400 {object} shared.Response
// @Failure 401 {object} shared.Response
// @Failure 403 {object} shared.Response
// @Failure 404 {object} shared.Response
// @Failure 500 {object} shared.Response
// @Router /api/admin/users/{id} [delete]
func deleteUserHandler(state *State) fiber.Handler {
	return func(c fiber.Ctx) error {
		service, err := requireUserService(c, state)
		if err != nil {
			return err
		}

		userID, err := parseUserID(c)
		if err != nil {
			return err
		}

		actorID, _ := c.Locals(localUserID).(uint)
		if err := service.DeleteUser(actorID, userID); err != nil {
			return writeUserServiceError(c, err, "DELETE_USER_FAILED", "failed to delete user")
		}
		return c.JSON(shared.OK(map[string]any{"deleted": true}))
	}
}

func requireUserService(c fiber.Ctx, state *State) (*user.Service, error) {
	service := state.UserService()
	if service == nil {
		return nil, c.Status(fiber.StatusServiceUnavailable).JSON(shared.Err("USERS_UNAVAILABLE", "user service unavailable"))
	}
	return service, nil
}

func parseUserID(c fiber.Ctx) (uint, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id == 0 {
		return 0, badRequest(c, "invalid user id")
	}
	return uint(id), nil
}

func parseOptionalInt(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func badRequest(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(shared.Err("BAD_REQUEST", message))
}

func writeUserServiceError(c fiber.Ctx, err error, internalCode string, internalMessage string) error {
	status, code, message := mapUserServiceError(err, internalCode, internalMessage)
	return c.Status(status).JSON(shared.Err(code, message))
}

func mapUserServiceError(err error, internalCode string, internalMessage string) (int, string, string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.StatusNotFound, "USER_NOT_FOUND", "user not found"
	}
	if errors.Is(err, user.ErrDuplicateUsername) {
		return fiber.StatusConflict, "DUPLICATE_USERNAME", user.ErrDuplicateUsername.Error()
	}
	if errors.Is(err, user.ErrDuplicateEmail) {
		return fiber.StatusConflict, "DUPLICATE_EMAIL", user.ErrDuplicateEmail.Error()
	}
	if errors.Is(err, user.ErrUsernameRequired) {
		return fiber.StatusBadRequest, "USERNAME_REQUIRED", user.ErrUsernameRequired.Error()
	}
	if errors.Is(err, user.ErrEmailRequired) {
		return fiber.StatusBadRequest, "EMAIL_REQUIRED", user.ErrEmailRequired.Error()
	}
	if errors.Is(err, user.ErrPasswordTooShort) {
		return fiber.StatusBadRequest, "PASSWORD_TOO_SHORT", user.ErrPasswordTooShort.Error()
	}
	if errors.Is(err, user.ErrInvalidRole) {
		return fiber.StatusBadRequest, "INVALID_ROLE", user.ErrInvalidRole.Error()
	}
	if errors.Is(err, user.ErrInvalidStatus) {
		return fiber.StatusBadRequest, "INVALID_STATUS", user.ErrInvalidStatus.Error()
	}
	if errors.Is(err, user.ErrCannotDisableSelf) {
		return fiber.StatusBadRequest, "CANNOT_DISABLE_SELF", user.ErrCannotDisableSelf.Error()
	}
	if errors.Is(err, user.ErrCannotDeleteSelf) {
		return fiber.StatusBadRequest, "CANNOT_DELETE_SELF", user.ErrCannotDeleteSelf.Error()
	}
	if errors.Is(err, user.ErrInvalidOldPassword) {
		return fiber.StatusBadRequest, "INVALID_OLD_PASSWORD", user.ErrInvalidOldPassword.Error()
	}
	if errors.Is(err, user.ErrPasswordConfirmationMismatch) {
		return fiber.StatusBadRequest, "PASSWORD_CONFIRMATION_MISMATCH", user.ErrPasswordConfirmationMismatch.Error()
	}
	return fiber.StatusInternalServerError, internalCode, internalMessage
}
