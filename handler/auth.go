package handler

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/validator"
	"github.com/ysicing/go-template/store"
)

type authUserStore interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUsernameOrEmail(ctx context.Context, identity string) (*model.User, error)
	GetByInviteCode(ctx context.Context, inviteCode string) (*model.User, error)
}

type authRefreshTokenStore interface {
	Create(ctx context.Context, rt *model.APIRefreshToken) error
	ConsumeToken(ctx context.Context, hash string) (*model.APIRefreshToken, error)
	DeleteByFamily(ctx context.Context, family string) error
	DeleteByTokenHash(ctx context.Context, hash string) error
}

// AuthDeps aggregates dependencies required by AuthHandler.
type AuthDeps struct {
	Users         authUserStore
	WebAuthnCreds *store.WebAuthnStore
	RefreshTokens authRefreshTokenStore
	Sessions      *service.SessionService
	AuthService   *service.AuthService
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
	webauthnCreds *store.WebAuthnStore
	refreshTokens authRefreshTokenStore
	sessions      *service.SessionService
	authService   *service.AuthService
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
		sessions = service.NewSessionService(deps.RefreshTokens, service.TokenConfig{
			Secret:        deps.TokenConfig.Secret,
			Issuer:        deps.TokenConfig.Issuer,
			AccessTTL:     deps.TokenConfig.AccessTTL,
			RefreshTTL:    deps.TokenConfig.RefreshTTL,
			RememberMeTTL: deps.TokenConfig.RememberMeTTL,
		})
	}
	authService := deps.AuthService
	if authService == nil {
		authService = service.NewAuthService(service.AuthServiceDeps{
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

const (
	lockoutThreshold = 5
	lockoutTTL       = 15 * time.Minute
)

// dummyHash is a pre-computed bcrypt hash used for constant-time comparison
// when a user is not found, preventing timing-based user enumeration.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), 12)

func loginFailKey(userID string) string { return "login_fail:" + userID }
func loginLockKey(userID string) string { return "login_lock:" + userID }

// isAccountLocked checks if a user account is locked.
func isAccountLocked(ctx context.Context, cache store.Cache, userID string) bool {
	val, err := cache.Get(ctx, loginLockKey(userID))
	return err == nil && val != ""
}

// recordFailedAuthAttempt atomically increments the failure counter and locks if threshold reached.
func recordFailedAuthAttempt(ctx context.Context, cache store.Cache, userID string) {
	key := loginFailKey(userID)
	count, _ := cache.Incr(ctx, key, lockoutTTL)
	if count >= int64(lockoutThreshold) {
		_ = cache.Set(ctx, loginLockKey(userID), "1", lockoutTTL)
	}
}

// clearFailedAuthAttempts resets the failure counter on successful login.
func clearFailedAuthAttempts(ctx context.Context, cache store.Cache, userID string) {
	_ = cache.Del(ctx, loginFailKey(userID))
	_ = cache.Del(ctx, loginLockKey(userID))
}

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

// Register handles POST /api/auth/register.
func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		InviteCode string `json:"invite_code"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.InviteCode = strings.TrimSpace(req.InviteCode)

	if req.Username == "" || req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username, email and password are required"})
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username must be 3-32 characters"})
	}
	if !isValidEmail(req.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid email format"})
	}

	// 邮箱域名验证
	mode := h.settings.Get(store.SettingEmailDomainMode, "disabled")
	if mode != "disabled" {
		whitelist := h.settings.GetStringSlice(store.SettingEmailDomainWhitelist, nil)
		blacklist := h.settings.GetStringSlice(store.SettingEmailDomainBlacklist, nil)
		if err := validator.ValidateEmailDomain(req.Email, mode, whitelist, blacklist); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email domain not allowed"})
		}
	}

	if shouldEnforcePasswordPolicy(h.settings) {
		if err := model.ValidatePasswordStrength(req.Password); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}

	var invitedByUserID string
	if req.InviteCode != "" {
		inviter, err := h.users.GetByInviteCode(c.Context(), req.InviteCode)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid invite code"})
		}
		invitedByUserID = inviter.ID
	}

	inviteCode := uuid.NewString()
	if len(inviteCode) > 32 {
		inviteCode = inviteCode[:32]
	}

	user := &model.User{
		Username:        req.Username,
		Email:           req.Email,
		Provider:        "local",
		ProviderID:      req.Username, // Use username as ProviderID for local accounts
		InviteCode:      inviteCode,
		InvitedByUserID: invitedByUserID,
	}
	if invitedByUserID != "" {
		user.InviteIP = GetRealIP(c)
	}
	if err := user.SetPassword(req.Password); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
	}

	if err := h.users.Create(c.Context(), user); err != nil {
		if store.IsUniqueViolation(err) {
			RecordAuthAttempt("register", "failure")
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "username or email already exists"})
		}
		RecordAuthAttempt("register", "failure")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create user"})
	}

	h.recordAudit(c, user.ID, model.AuditRegister, "user", user.ID, "success", "")
	RecordAuthAttempt("register", "success")

	resp := fiber.Map{
		"user": user,
	}

	// Send verification email if enabled.
	if h.settings != nil && h.settings.GetBool(store.SettingEmailVerificationEnabled, false) && h.emailHandler != nil {
		baseURL := c.Protocol() + "://" + c.Hostname()
		// Email is sent asynchronously, so this won't block the response
		_ = h.emailHandler.SendVerificationEmail(c, user, baseURL)
		resp["email_verification_required"] = true
	}

	issuedSession, err := h.sessions.IssueBrowserSession(c.Context(), service.SessionRequest{
		User:       user,
		IP:         GetRealIP(c),
		UserAgent:  c.Get("User-Agent"),
		RefreshTTL: h.tokenConfig.RefreshTTL,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}
	// Note: tokens are NOT returned in JSON for security (XSS protection)
	// They are set as HttpOnly cookies instead

	// Set tokens in cookies for web clients
	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, h.tokenConfig.RefreshTTL)

	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		RememberMe bool   `json:"remember_me"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username and password are required"})
	}

	user, err := h.authService.Login(c.Context(), service.LoginInput{
		Identity: req.Username,
		Password: req.Password,
	})
	if errors.Is(err, service.ErrAccountLocked) {
		h.recordAudit(c, user.ID, model.AuditLoginFailed, "user", user.ID, "failure", "account locked")
		RecordAuthAttempt("login", "failure")
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}
	if errors.Is(err, service.ErrInvalidCredentials) {
		RecordAuthAttempt("login", "failure")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "login failed"})
	}

	// Determine refresh token TTL based on remember_me.
	refreshTTL := h.tokenConfig.RefreshTTL
	if req.RememberMe {
		refreshTTL = h.tokenConfig.RememberMeTTL
	}

	// Check if MFA is enabled (TOTP only, WebAuthn is a separate passwordless flow).
	mfaCfg, _ := h.mfa.GetByUserID(c.Context(), user.ID)
	if mfaCfg != nil && mfaCfg.TOTPEnabled {
		mfaToken := store.GenerateRandomToken()
		_ = h.cache.Set(c.Context(), "mfa_pending:"+mfaToken, user.ID, 5*time.Minute)
		if req.RememberMe {
			_ = h.cache.Set(c.Context(), "mfa_pending_rm:"+mfaToken, "1", 5*time.Minute)
		}
		return c.JSON(fiber.Map{
			"mfa_required": true,
			"mfa_token":    mfaToken,
		})
	}

	h.recordAudit(c, user.ID, model.AuditLogin, "user", user.ID, "success", "local")
	RecordAuthAttempt("login", "success")

	issuedSession, err := h.sessions.IssueBrowserSession(c.Context(), service.SessionRequest{
		User:       user,
		IP:         GetRealIP(c),
		UserAgent:  c.Get("User-Agent"),
		RefreshTTL: refreshTTL,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}

	if req.RememberMe {
		tokenHash := store.HashToken(issuedSession.RefreshToken)
		_ = h.cache.Set(c.Context(), rtRememberKey(tokenHash), "1", h.tokenConfig.RememberMeTTL)
	}

	// Set tokens in cookies for web clients
	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, refreshTTL)

	// Note: tokens are NOT returned in JSON for security (XSS protection)
	return c.JSON(fiber.Map{
		"user": user,
	})
}

// Refresh handles POST /api/auth/refresh.
// Reads refresh_token from HttpOnly cookie for security.
func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	// Read refresh token from HttpOnly cookie (not from request body for security)
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing refresh token"})
	}

	tokenHash := store.HashToken(refreshToken)

	// Atomically consume the refresh token (get + delete in transaction)
	// This prevents concurrent refresh requests from reusing the same token
	rt, err := h.refreshTokens.ConsumeToken(c.Context(), tokenHash)
	if err != nil {
		// Token not found or already consumed — possible replay attack
		// Check if we have a cached family for this hash
		if family, cacheErr := h.cache.Get(c.Context(), "rt_used:"+tokenHash); cacheErr == nil && family != "" {
			// Replay detected: revoke the entire token family
			_ = h.refreshTokens.DeleteByFamily(c.Context(), family)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid refresh token"})
	}

	if time.Now().After(rt.ExpiresAt) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "refresh token expired"})
	}

	family := rt.Family

	// Cache hash→family for replay detection
	_ = h.cache.Set(c.Context(), "rt_used:"+tokenHash, family, h.tokenConfig.RememberMeTTL)

	user, err := h.users.GetByID(c.Context(), rt.UserID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	// Use the original TTL from token creation, not remaining time
	// This prevents TTL from decreasing on each refresh
	refreshTTL := h.tokenConfig.RefreshTTL

	// Check if this was a "remember me" token
	if rmVal, _ := h.cache.Get(c.Context(), rtRememberKey(tokenHash)); rmVal == "1" {
		refreshTTL = h.tokenConfig.RememberMeTTL
	}

	issuedSession, err := h.sessions.RotateBrowserSession(c.Context(), service.SessionRequest{
		User:       user,
		IP:         GetRealIP(c),
		UserAgent:  c.Get("User-Agent"),
		RefreshTTL: refreshTTL,
		Family:     family,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}

	// Propagate remember_me flag to the new token
	if refreshTTL == h.tokenConfig.RememberMeTTL {
		_ = h.cache.Del(c.Context(), rtRememberKey(tokenHash))
		newTokenHash := store.HashToken(issuedSession.RefreshToken)
		_ = h.cache.Set(c.Context(), rtRememberKey(newTokenHash), "1", h.tokenConfig.RememberMeTTL)
	}

	// Set tokens in cookies for web clients
	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, refreshTTL)

	// Note: tokens are NOT returned in JSON for security (XSS protection)
	return c.JSON(fiber.Map{
		"message": "tokens refreshed",
	})
}

// Logout handles POST /api/auth/logout.
func (h *AuthHandler) Logout(c fiber.Ctx) error {
	// Read refresh token from HttpOnly cookie only (never from body to prevent token leakage)
	refreshToken := c.Cookies("refresh_token")

	if refreshToken != "" {
		tokenHash := store.HashToken(refreshToken)
		_ = h.refreshTokens.DeleteByTokenHash(c.Context(), tokenHash)
	}

	// Always clear token cookies
	ClearTokenCookies(c)

	userID, _ := c.Locals("user_id").(string)
	if userID != "" {
		h.recordAudit(c, userID, model.AuditLogout, "user", userID, "success", "")
	}
	return c.JSON(fiber.Map{"message": "logged out"})
}
