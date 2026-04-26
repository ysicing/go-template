package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
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
	return c.JSON(fiber.Map{"user": NewUserResponse(user)})
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
		if errors.Is(err, store.ErrNotFound) {
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
		if errors.Is(err, store.ErrNotFound) {
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
