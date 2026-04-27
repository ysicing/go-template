package handler

import (
	"context"
	"net/mail"
	"strings"

	authservice "github.com/ysicing/go-template/internal/service/auth"
	sessionservice "github.com/ysicing/go-template/internal/service/session"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
	webauthnstore "github.com/ysicing/go-template/store/webauthn"

	"github.com/gofiber/fiber/v3"
)

type authUserStore interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUsernameOrEmail(ctx context.Context, identity string) (*model.User, error)
	GetByInviteCode(ctx context.Context, inviteCode string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
}

type authRefreshTokenStore interface {
	Create(ctx context.Context, rt *model.APIRefreshToken) error
	ConsumeToken(ctx context.Context, hash string) (*model.APIRefreshToken, error)
	DeleteByFamily(ctx context.Context, family string) error
	DeleteByTokenHash(ctx context.Context, hash string) error
	GetUsedFamily(ctx context.Context, hash string) (string, error)
}

// AuthDeps aggregates dependencies required by AuthHandler.
type AuthDeps struct {
	Users         authUserStore
	WebAuthnCreds *webauthnstore.WebAuthnStore
	RefreshTokens authRefreshTokenStore
	Sessions      *sessionservice.SessionService
	AuthService   *authservice.AuthService
	MFA           mfaReader
	Audit         *store.AuditLogStore
	Cache         store.Cache
	Settings      settingReader
	EmailHandler  *EmailHandler
	TokenConfig   TokenConfig
}

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	users         authUserStore
	webauthnCreds *webauthnstore.WebAuthnStore
	refreshTokens authRefreshTokenStore
	sessions      *sessionservice.SessionService
	authService   *authservice.AuthService
	mfa           mfaReader
	audit         *store.AuditLogStore
	cache         store.Cache
	settings      settingReader
	emailHandler  *EmailHandler
	tokenConfig   TokenConfig
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(deps AuthDeps) *AuthHandler {
	sessions := deps.Sessions
	if sessions == nil {
		sessions = sessionservice.NewSessionService(deps.RefreshTokens, deps.TokenConfig.ToServiceConfig())
	}
	authService := deps.AuthService
	if authService == nil {
		authService = authservice.NewAuthService(authservice.AuthServiceDeps{
			Users: deps.Users,
			Cache: deps.Cache,
		})
	}

	return &AuthHandler{
		users:         deps.Users,
		webauthnCreds: deps.WebAuthnCreds,
		refreshTokens: deps.RefreshTokens,
		sessions:      sessions,
		authService:   authService,
		mfa:           deps.MFA,
		audit:         deps.Audit,
		cache:         deps.Cache,
		settings:      deps.Settings,
		emailHandler:  deps.EmailHandler,
		tokenConfig:   deps.TokenConfig,
	}
}

// SetEmailHandler sets the email handler (used to break init cycle in router).
func (h *AuthHandler) SetEmailHandler(eh *EmailHandler) {
	h.emailHandler = eh
}

func rtRememberKey(tokenHash string) string { return "rt_remember:" + tokenHash }

// isValidEmail performs basic email format validation.
func isValidEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	parts := strings.SplitN(parsed.Address, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := parts[1]
	return domain != "" && strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}

func (h *AuthHandler) recordAudit(c fiber.Ctx, userID, action, resource, resourceID, status, detail string) {
	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		IP:         ip,
		UserAgent:  ua,
		Status:     status,
		Detail:     detail,
	})
}

// SetupPassword handles POST /api/auth/setup-password for one-time password setup tokens.
func (h *AuthHandler) SetupPassword(c fiber.Ctx) error {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "token and password are required"})
	}
	if shouldEnforcePasswordPolicy(h.settings) {
		if err := model.ValidatePasswordStrength(req.Password); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}

	ephemeral := store.NewEphemeralTokenStore(h.cache)
	userID, err := ephemeral.ConsumeString(c.Context(), "password_setup", "user", req.Token)
	if err != nil || userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid or expired token"})
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user not found"})
	}
	if user.PasswordHash != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password already set"})
	}
	if err := user.SetPassword(req.Password); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
	}
	if err := h.users.Update(c.Context(), user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to set password"})
	}

	h.recordAudit(c, user.ID, model.AuditPasswordSet, "user", user.ID, "success", "password set via setup token")
	return c.JSON(fiber.Map{"message": "password set successfully"})
}

// Logout handles POST /api/auth/logout.
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")
	if refreshToken != "" {
		tokenHash := store.HashToken(refreshToken)
		_ = h.refreshTokens.DeleteByTokenHash(c.Context(), tokenHash)
	}

	ClearTokenCookies(c)

	userID, _ := c.Locals("user_id").(string)
	if userID != "" {
		h.recordAudit(c, userID, model.AuditLogout, "user", userID, "success", "")
	}
	return c.JSON(fiber.Map{"message": "logged out"})
}
