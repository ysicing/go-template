package handler

import (
	"errors"
	"strings"
	"time"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(c fiber.Ctx) error {
	req, err := parseLoginRequest(c)
	if err != nil {
		return finishHandlerError(c, err)
	}

	user, err := h.loginUser(c, req)
	if err != nil {
		return finishHandlerError(c, err)
	}

	refreshTTL := h.loginRefreshTTL(req.RememberMe)
	if mfaResp, ok, err := h.tryBeginMFALogin(c, user, req.RememberMe); ok || err != nil {
		if err != nil {
			return finishHandlerError(c, err)
		}
		return c.JSON(mfaResp)
	}

	h.recordAudit(c, user.ID, model.AuditLogin, "user", user.ID, "success", "local")
	RecordAuthAttempt("login", "success")
	return h.finishLogin(c, user, refreshTTL, req.RememberMe)
}

func parseLoginRequest(c fiber.Ctx) (*loginRequest, error) {
	var req loginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return nil, jsonError(fiber.StatusBadRequest, "invalid request body")
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		return nil, jsonError(fiber.StatusBadRequest, "username and password are required")
	}
	return &req, nil
}

func (h *AuthHandler) loginUser(c fiber.Ctx, req *loginRequest) (*model.User, error) {
	user, err := h.authService.Login(c.Context(), service.LoginInput{
		Identity: req.Username,
		Password: req.Password,
	})
	if errors.Is(err, service.ErrAccountLocked) {
		if user != nil {
			h.recordAudit(c, user.ID, model.AuditLoginFailed, "user", user.ID, "failure", "account locked")
		}
		RecordAuthAttempt("login", "failure")
		return nil, jsonError(fiber.StatusTooManyRequests, "account temporarily locked, try again later")
	}
	if errors.Is(err, service.ErrInvalidCredentials) {
		RecordAuthAttempt("login", "failure")
		return nil, jsonError(fiber.StatusUnauthorized, "invalid credentials")
	}
	if err != nil {
		return nil, jsonError(fiber.StatusInternalServerError, "login failed")
	}
	return user, nil
}

func (h *AuthHandler) loginRefreshTTL(rememberMe bool) time.Duration {
	if rememberMe {
		return h.tokenConfig.RememberMeTTL
	}
	return h.tokenConfig.RefreshTTL
}

func (h *AuthHandler) tryBeginMFALogin(c fiber.Ctx, user *model.User, rememberMe bool) (fiber.Map, bool, error) {
	mfaCfg, _ := h.mfa.GetByUserID(c.Context(), user.ID)
	if mfaCfg == nil || !mfaCfg.TOTPEnabled {
		return nil, false, nil
	}

	mfaToken := store.GenerateRandomToken()
	if err := h.cache.Set(c.Context(), "mfa_pending:"+mfaToken, user.ID, 5*time.Minute); err != nil {
		return nil, true, jsonError(fiber.StatusInternalServerError, "failed to initiate MFA")
	}
	if rememberMe {
		_ = h.cache.Set(c.Context(), "mfa_pending_rm:"+mfaToken, "1", 5*time.Minute)
	}
	return fiber.Map{"mfa_required": true, "mfa_token": mfaToken}, true, nil
}

func (h *AuthHandler) finishLogin(c fiber.Ctx, user *model.User, refreshTTL time.Duration, rememberMe bool) error {
	issuedSession, err := h.sessions.IssueBrowserSession(c.Context(), browserSessionRequest(c, user, refreshTTL))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}
	if rememberMe {
		tokenHash := store.HashToken(issuedSession.RefreshToken)
		_ = h.cache.Set(c.Context(), rtRememberKey(tokenHash), "1", h.tokenConfig.RememberMeTTL)
	}
	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, refreshTTL)
	return c.JSON(fiber.Map{"user": NewUserResponse(user)})
}

// Refresh handles POST /api/auth/refresh.
// Reads refresh_token from HttpOnly cookie for security.
func (h *AuthHandler) Refresh(c fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing refresh token"})
	}

	rt, tokenHash, err := h.consumeRefreshToken(c, refreshToken)
	if err != nil {
		return finishHandlerError(c, err)
	}
	if time.Now().After(rt.ExpiresAt) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "refresh token expired"})
	}

	user, err := h.users.GetByID(c.Context(), rt.UserID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	refreshTTL := h.refreshTTLForToken(c, tokenHash)
	return h.finishRefresh(c, user, rt.Family, tokenHash, refreshTTL)
}

func (h *AuthHandler) consumeRefreshToken(c fiber.Ctx, refreshToken string) (*model.APIRefreshToken, string, error) {
	tokenHash := store.HashToken(refreshToken)
	if err := h.revokeRefreshFamilyIfUsed(c, tokenHash); err != nil {
		return nil, "", err
	}

	rt, err := h.refreshTokens.ConsumeToken(c.Context(), tokenHash)
	if err != nil {
		_ = h.revokeRefreshFamilyIfUsed(c, tokenHash)
		return nil, "", jsonError(fiber.StatusUnauthorized, "invalid refresh token")
	}
	return rt, tokenHash, nil
}

func (h *AuthHandler) revokeRefreshFamilyIfUsed(c fiber.Ctx, tokenHash string) error {
	family, err := h.refreshTokens.GetUsedFamily(c.Context(), tokenHash)
	if err == nil && family != "" {
		_ = h.refreshTokens.DeleteByFamily(c.Context(), family)
		return jsonError(fiber.StatusUnauthorized, "invalid refresh token")
	}
	return nil
}

func (h *AuthHandler) refreshTTLForToken(c fiber.Ctx, tokenHash string) time.Duration {
	if rmVal, _ := h.cache.Get(c.Context(), rtRememberKey(tokenHash)); rmVal == "1" {
		return h.tokenConfig.RememberMeTTL
	}
	return h.tokenConfig.RefreshTTL
}

func (h *AuthHandler) finishRefresh(c fiber.Ctx, user *model.User, family, oldTokenHash string, refreshTTL time.Duration) error {
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

	if refreshTTL == h.tokenConfig.RememberMeTTL {
		_ = h.cache.Del(c.Context(), rtRememberKey(oldTokenHash))
		newTokenHash := store.HashToken(issuedSession.RefreshToken)
		_ = h.cache.Set(c.Context(), rtRememberKey(newTokenHash), "1", h.tokenConfig.RememberMeTTL)
	}

	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, refreshTTL)
	return c.JSON(fiber.Map{"message": "tokens refreshed"})
}

func browserSessionRequest(c fiber.Ctx, user *model.User, refreshTTL time.Duration) service.SessionRequest {
	return service.SessionRequest{
		User:       user,
		IP:         GetRealIP(c),
		UserAgent:  c.Get("User-Agent"),
		RefreshTTL: refreshTTL,
	}
}
