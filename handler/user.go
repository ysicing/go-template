package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/validator"
	"github.com/ysicing/go-template/store"
)

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

type userConsentGrantStore interface {
	ListByUserIDPaged(ctx context.Context, userID string, page, pageSize int) ([]model.OAuthConsentGrant, int64, error)
	DeleteByIDAndUserID(ctx context.Context, id, userID string) error
}

type userOAuthClientStore interface {
	GetByClientID(ctx context.Context, clientID string) (*model.OAuthClient, error)
}

// UserDeps aggregates dependencies required by UserHandler.
type UserDeps struct {
	Users           userStore
	PasswordHistory passwordHistoryStore
	RefreshTokens   userRefreshTokenStore
	Audit           userAuditLogStore
	ConsentGrants   userConsentGrantStore
	Clients         userOAuthClientStore
	Settings        settingReader
	EmailHandler    *EmailHandler
	Cache           store.Cache
}

// UserHandler handles user profile endpoints.
type UserHandler struct {
	users           userStore
	passwordHistory passwordHistoryStore
	refreshTokens   userRefreshTokenStore
	audit           userAuditLogStore
	consentGrants   userConsentGrantStore
	clients         userOAuthClientStore
	settings        settingReader
	emailHandler    *EmailHandler
	cache           store.Cache
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(deps UserDeps) *UserHandler {
	return &UserHandler{
		users:           deps.Users,
		passwordHistory: deps.PasswordHistory,
		refreshTokens:   deps.RefreshTokens,
		audit:           deps.Audit,
		consentGrants:   deps.ConsentGrants,
		clients:         deps.Clients,
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
	return c.JSON(fiber.Map{"user": user})
}

// UpdateMe handles PUT /api/users/me.
func (h *UserHandler) UpdateMe(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	var req struct {
		Username  *string `json:"username"`
		Email     *string `json:"email"`
		AvatarURL *string `json:"avatar_url"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	emailChanged := false

	if req.Username != nil {
		u := strings.TrimSpace(*req.Username)
		if len(u) < 3 || len(u) > 32 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username must be 3-32 characters"})
		}
		user.Username = u
	}
	if req.Email != nil {
		e := strings.TrimSpace(*req.Email)
		if !strings.Contains(e, "@") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid email format"})
		}
		if e != user.Email {
			// 频率限制：30 天一次
			if user.EmailUpdatedAt != nil {
				daysSince := time.Since(*user.EmailUpdatedAt).Hours() / 24
				if daysSince < 30 {
					remaining := int(30 - daysSince)
					return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
						"error": fmt.Sprintf("email can only be changed once every 30 days, %d days remaining", remaining),
					})
				}
			}

			// 检查邮箱是否已被使用
			if existing, _ := h.users.GetByEmail(c.Context(), e); existing != nil && existing.ID != user.ID {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "email already in use"})
			}

			// 邮箱域名验证
			if h.settings != nil {
				mode := h.settings.Get(store.SettingEmailDomainMode, "disabled")
				if mode != "disabled" {
					whitelist := h.settings.GetStringSlice(store.SettingEmailDomainWhitelist, nil)
					blacklist := h.settings.GetStringSlice(store.SettingEmailDomainBlacklist, nil)
					if err := validator.ValidateEmailDomain(e, mode, whitelist, blacklist); err != nil {
						return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email domain not allowed"})
					}
				}
			}

			now := time.Now()
			user.Email = e
			user.EmailVerified = false
			user.EmailUpdatedAt = &now
			emailChanged = true
		}
	}
	if req.AvatarURL != nil {
		user.AvatarURL = *req.AvatarURL
	}

	if err := h.users.Update(c.Context(), user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update user"})
	}

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditUserUpdate,
		Resource:   "user",
		ResourceID: userID,
		Status:     "success",
		Detail:     "user profile updated",
	})

	// 如果邮箱已更改，发送验证邮件
	if emailChanged && h.emailHandler != nil &&
		h.settings != nil && h.settings.GetBool(store.SettingEmailVerificationEnabled, false) {
		baseURL := c.Protocol() + "://" + c.Hostname()
		_ = h.emailHandler.SendVerificationEmail(c, user, baseURL)

		_ = recordAuditFromFiber(c, h.audit, AuditEvent{
			UserID:     userID,
			Action:     model.AuditEmailChange,
			Resource:   "user",
			ResourceID: userID,
			Status:     "success",
			Detail:     "user email changed",
			Metadata: map[string]string{
				"email": user.Email,
			},
		})
	}

	return c.JSON(fiber.Map{"user": user})
}

// ChangePassword handles PUT /api/users/me/password.
func (h *UserHandler) ChangePassword(c fiber.Ctx) error {
	const historyKeep = 5

	userID, _ := c.Locals("user_id").(string)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	if user.PasswordHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no password set, use set-password first"})
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "current_password and new_password are required"})
	}
	if !user.CheckPassword(req.CurrentPassword) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid current password"})
	}
	if user.CheckPassword(req.NewPassword) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "new password must be different from current password"})
	}
	if err := model.ValidatePasswordStrength(req.NewPassword); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	reused, err := h.passwordHistory.IsRecentlyUsed(c.Context(), userID, req.NewPassword, historyKeep)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to validate password history"})
	}
	if reused {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "new password was used recently"})
	}

	// Save old password hash BEFORE setting new password
	oldPasswordHash := user.PasswordHash

	if err := user.SetPassword(req.NewPassword); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
	}
	if user.TokenVersion < 1 {
		user.TokenVersion = 1
	}
	user.TokenVersion++
	// Record old password hash in history, not new one
	if err := h.users.ChangePasswordWithHistory(c.Context(), user, oldPasswordHash); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update password"})
	}

	_ = h.passwordHistory.TrimByUserID(c.Context(), userID, historyKeep)
	_ = h.refreshTokens.DeleteByUserID(c.Context(), userID)
	if h.cache != nil {
		_ = h.cache.Del(c.Context(), "token_ver:"+userID)
	}

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditPasswordChange,
		Resource:   "user",
		ResourceID: userID,
		Status:     "success",
		Detail:     "password changed",
	})

	return c.JSON(fiber.Map{"message": "password updated, please login again"})
}

// ListSessions handles GET /api/sessions/.
func (h *UserHandler) ListSessions(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	page, pageSize := parsePagination(c)
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke session"})
	}
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditSessionRevoke,
		Resource:   "session",
		ResourceID: sessionID,
		Status:     "success",
		Detail:     "session revoked",
	})
	return c.JSON(fiber.Map{"message": "session revoked"})
}

// RevokeAllSessions handles DELETE /api/sessions/.
func (h *UserHandler) RevokeAllSessions(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	if err := h.refreshTokens.DeleteByUserID(c.Context(), userID); err != nil {
		_ = recordAuditFromFiber(c, h.audit, AuditEvent{
			UserID:   userID,
			Action:   model.AuditSessionRevoke,
			Resource: "session",
			Status:   "failure",
			Detail:   "revoke all sessions failed",
			Metadata: map[string]string{
				"scope":  "all_sessions",
				"reason": "refresh_tokens_revoke_failed",
			},
		})
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke all sessions"})
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		_ = recordAuditFromFiber(c, h.audit, AuditEvent{
			UserID:   userID,
			Action:   model.AuditSessionRevoke,
			Resource: "session",
			Status:   "failure",
			Detail:   "revoke all sessions failed",
			Metadata: map[string]string{
				"scope":  "all_sessions",
				"reason": "user_not_found",
			},
		})
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke all sessions"})
	}
	if user.TokenVersion < 1 {
		user.TokenVersion = 1
	}
	user.TokenVersion++
	if err := h.users.Update(c.Context(), user); err != nil {
		_ = recordAuditFromFiber(c, h.audit, AuditEvent{
			UserID:   userID,
			Action:   model.AuditSessionRevoke,
			Resource: "session",
			Status:   "failure",
			Detail:   "revoke all sessions failed",
			Metadata: map[string]string{
				"scope":  "all_sessions",
				"reason": "token_version_update_failed",
			},
		})
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke all sessions"})
	}
	if h.cache != nil {
		_ = h.cache.Del(c.Context(), "token_ver:"+userID)
	}
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:   userID,
		Action:   model.AuditSessionRevoke,
		Resource: "session",
		Status:   "success",
		Detail:   "all sessions revoked",
		Metadata: map[string]string{
			"scope": "all_sessions",
		},
	})
	return c.JSON(fiber.Map{"message": "all sessions revoked"})
}

// GetLoginHistory handles GET /api/users/me/login-history.
func (h *UserHandler) GetLoginHistory(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	page, pageSize := parsePagination(c)
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

// ListAuthorizedApps handles GET /api/users/me/authorized-apps.
func (h *UserHandler) ListAuthorizedApps(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	page, pageSize := parsePagination(c)
	grants, total, err := h.consentGrants.ListByUserIDPaged(c.Context(), userID, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list authorized apps"})
	}

	type authorizedAppResp struct {
		ID         string `json:"id"`
		ClientID   string `json:"client_id"`
		ClientName string `json:"client_name"`
		Scopes     string `json:"scopes"`
		GrantedAt  string `json:"granted_at"`
	}
	apps := make([]authorizedAppResp, len(grants))
	for index, grant := range grants {
		clientName := grant.ClientID
		if h.clients != nil {
			client, clientErr := h.clients.GetByClientID(c.Context(), grant.ClientID)
			if clientErr == nil && client != nil && strings.TrimSpace(client.Name) != "" {
				clientName = client.Name
			}
		}
		apps[index] = authorizedAppResp{
			ID:         grant.ID,
			ClientID:   grant.ClientID,
			ClientName: clientName,
			Scopes:     grant.Scopes,
			GrantedAt:  grant.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return c.JSON(fiber.Map{
		"apps":      apps,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// RevokeAuthorizedApp handles DELETE /api/users/me/authorized-apps/:id.
func (h *UserHandler) RevokeAuthorizedApp(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	grantID := c.Params("id")
	if err := h.consentGrants.DeleteByIDAndUserID(c.Context(), grantID, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "authorized app not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke authorized app"})
	}

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditOIDCConsentGrantRevoke,
		Resource:   "oauth_consent_grant",
		ResourceID: grantID,
		Status:     "success",
		Detail:     "authorized app revoked",
	})

	return c.JSON(fiber.Map{"message": "authorized app revoked"})
}

// SetPassword handles POST /api/users/me/set-password (OAuth users setting initial password).
func (h *UserHandler) SetPassword(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	// Only allow if user has no password set
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
	if err := model.ValidatePasswordStrength(req.Password); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := user.SetPassword(req.Password); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
	}
	if err := h.users.Update(c.Context(), user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to set password"})
	}

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditPasswordSet,
		Resource:   "user",
		ResourceID: userID,
		Status:     "success",
		Detail:     "password set",
	})

	return c.JSON(fiber.Map{"message": "password set successfully"})
}
