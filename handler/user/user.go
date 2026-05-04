package user

import (
	"context"
	"errors"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

type settingReader interface {
	Get(key, defaultVal string) string
	GetBool(key string, defaultVal bool) bool
	GetInt(key string, defaultVal int) int
	GetStringSlice(key string, defaultVal []string) []string
}

type emailVerificationSender interface {
	SendVerificationEmail(c fiber.Ctx, user *model.User, baseURL string) error
}

type userStore interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	ChangePasswordWithHistory(ctx context.Context, user *model.User, previousPasswordHash string) error
}

type passwordHistoryStore interface {
	IsRecentlyUsed(ctx context.Context, userID, plaintext string, limit int) (bool, error)
	TrimByUserID(ctx context.Context, userID string, keep int) error
}

type userRefreshTokenStore interface {
	DeleteByUserID(ctx context.Context, userID string) error
	ListByUserIDPaged(ctx context.Context, userID string, page, pageSize int) ([]model.APIRefreshToken, int64, error)
	DeleteByIDAndUserID(ctx context.Context, id, userID string) error
}

type userAuditLogStore interface {
	Create(ctx context.Context, log *model.AuditLog) error
	ListLoginByUserIDPaged(ctx context.Context, userID string, page, pageSize int) ([]store.LoginRow, int64, error)
}

// UserDeps aggregates dependencies required by UserHandler.
type UserDeps struct {
	Users           userStore
	PasswordHistory passwordHistoryStore
	RefreshTokens   userRefreshTokenStore
	Audit           userAuditLogStore
	Settings        settingReader
	EmailHandler    emailVerificationSender
	Cache           store.Cache
}

// UserHandler handles user profile endpoints.
type UserHandler struct {
	users           userStore
	passwordHistory passwordHistoryStore
	refreshTokens   userRefreshTokenStore
	audit           userAuditLogStore
	settings        settingReader
	emailHandler    emailVerificationSender
	cache           store.Cache
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(deps UserDeps) *UserHandler {
	return &UserHandler{
		users:           deps.Users,
		passwordHistory: deps.PasswordHistory,
		refreshTokens:   deps.RefreshTokens,
		audit:           deps.Audit,
		settings:        deps.Settings,
		emailHandler:    deps.EmailHandler,
		cache:           deps.Cache,
	}
}

// GetMe handles GET /api/users/me.
func (h *UserHandler) GetMe(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return c.JSON(fiber.Map{"user": handlercommon.NewUserResponse(user)})
}

// ListSessions handles GET /api/sessions/.
func (h *UserHandler) ListSessions(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	page, pageSize := handlercommon.ParsePagination(c)
	tokens, total, err := h.refreshTokens.ListByUserIDPaged(c.Context(), userID, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list sessions"})
	}
	type sessionResp struct {
		ID         string `json:"id"`
		IP         string `json:"ip"`
		UserAgent  string `json:"user_agent"`
		LastUsedAt string `json:"last_used_at"`
		CreatedAt  string `json:"created_at"`
	}
	sessions := make([]sessionResp, len(tokens))
	for i, t := range tokens {
		sessions[i] = sessionResp{
			ID: t.ID, IP: t.IP, UserAgent: t.UserAgent,
			LastUsedAt: t.LastUsedAt.Format("2006-01-02T15:04:05Z"),
			CreatedAt:  t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return c.JSON(fiber.Map{
		"sessions":  sessions,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// RevokeSession handles DELETE /api/sessions/:id.
func (h *UserHandler) RevokeSession(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	sessionID := c.Params("id")
	if err := h.refreshTokens.DeleteByIDAndUserID(c.Context(), sessionID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke session"})
	}
	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
		UserID:     userID,
		Action:     model.AuditSessionRevoke,
		Resource:   "session",
		ResourceID: sessionID,
		Status:     "success",
		Detail:     "session revoked",
	})
	return c.JSON(fiber.Map{"message": "session revoked"})
}

// GetLoginHistory handles GET /api/users/me/login-history.
func (h *UserHandler) GetLoginHistory(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	page, pageSize := handlercommon.ParsePagination(c)
	rows, total, err := h.audit.ListLoginByUserIDPaged(c.Context(), userID, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list login history"})
	}
	return c.JSON(fiber.Map{
		"events":    rows,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// SetPassword handles POST /api/users/me/set-password (OAuth users setting initial password).
func (h *UserHandler) SetPassword(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	if user.PasswordHash != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password already set, use change password instead"})
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password is required"})
	}
	if shouldEnforcePasswordPolicy(h.settings) {
		if err := model.ValidatePasswordStrength(req.Password); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if err := user.SetPassword(req.Password); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
	}
	if err := h.users.Update(c.Context(), user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to set password"})
	}

	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
		UserID:     userID,
		Action:     model.AuditPasswordSet,
		Resource:   "user",
		ResourceID: userID,
		Status:     "success",
		Detail:     "password set",
	})

	return c.JSON(fiber.Map{"message": "password set successfully"})
}

func shouldEnforcePasswordPolicy(settings settingReader) bool {
	if settings == nil {
		return true
	}
	return settings.GetBool(store.SettingPasswordPolicyEnabled, true)
}
