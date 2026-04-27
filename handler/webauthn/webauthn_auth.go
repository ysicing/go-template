package webauthnhandler

import (
	"encoding/json"
	"time"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	rootstore "github.com/ysicing/go-template/store"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

// LoginBegin handles POST /api/auth/webauthn/begin (passwordless login).
func (h *WebAuthnHandler) LoginBegin(c fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username is required"})
	}
	user, err := h.lookupWebAuthnLoginUser(c, req.Username)
	if err != nil {
		return err
	}
	waUser, err := h.loadWebAuthnUser(c, user.ID)
	if err != nil || len(waUser.Creds) == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}
	wa, err := h.getWebAuthn()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}
	options, session, err := wa.BeginLogin(waUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to begin authentication"})
	}
	token := rootstore.GenerateRandomToken()
	sessionJSON, _ := json.Marshal(session)
	_ = h.cache.Set(c.Context(), "webauthn_login:"+token, user.ID+"|"+string(sessionJSON), webAuthnSessionTTL)
	return c.JSON(fiber.Map{"publicKey": options.Response, "webauthn_token": token})
}

// LoginFinish handles POST /api/auth/webauthn/finish (passwordless login).
func (h *WebAuthnHandler) LoginFinish(c fiber.Ctx) error {
	token, err := webAuthnTokenFromRequest(c, "X-WebAuthn-Token", "webauthn_token", "webauthn_token is required")
	if err != nil {
		return err
	}
	user, session, err := h.loadLoginAttempt(c, token)
	if err != nil {
		return err
	}
	credential, err := h.validateLoginResponse(c, user.ID, session)
	if err != nil {
		return h.handleLoginFailure(c, user.ID, token)
	}
	h.completeLoginSuccess(c, token, user.ID, credential.ID, credential.Authenticator.SignCount)
	if err := h.issueBrowserSession(c, user, h.tokenConfig.RefreshTTL); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}
	return c.JSON(fiber.Map{"user": handlercommon.NewUserResponse(user)})
}

// AuthBegin handles POST /api/auth/mfa/webauthn/begin.
func (h *WebAuthnHandler) AuthBegin(c fiber.Ctx) error {
	var req struct {
		MFAToken string `json:"mfa_token"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.MFAToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mfa_token is required"})
	}
	userID, err := h.cache.Get(c.Context(), "mfa_pending:"+req.MFAToken)
	if err != nil || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired mfa_token"})
	}
	waUser, err := h.loadWebAuthnUser(c, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	wa, err := h.getWebAuthn()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}
	options, session, err := wa.BeginLogin(waUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to begin authentication"})
	}
	h.saveSessionData(c.Context(), "webauthn_auth:"+req.MFAToken, session)
	return c.JSON(options)
}

// AuthFinish handles POST /api/auth/mfa/webauthn/finish.
func (h *WebAuthnHandler) AuthFinish(c fiber.Ctx) error {
	mfaToken, err := webAuthnTokenFromRequest(c, "X-MFA-Token", "mfa_token", "mfa_token query param is required")
	if err != nil {
		return err
	}
	userID, user, session, failKey, err := h.loadMFAAttempt(c, mfaToken)
	if err != nil {
		return err
	}
	credential, err := h.validateLoginResponse(c, userID, session)
	if err != nil {
		return h.handleMFAFailure(c, userID, failKey)
	}
	refreshTTL := h.completeMFASuccess(c, mfaToken, userID, failKey, credential.ID, credential.Authenticator.SignCount)
	if err := h.issueBrowserSession(c, user, refreshTTL); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}
	return c.JSON(fiber.Map{"user": handlercommon.NewUserResponse(user)})
}

func (h *WebAuthnHandler) lookupWebAuthnLoginUser(c fiber.Ctx, username string) (*model.User, error) {
	user, err := h.users.GetByUsername(c.Context(), username)
	if err != nil {
		user, err = h.users.GetByEmail(c.Context(), username)
		if err != nil {
			return nil, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
		}
	}
	if handlercommon.IsAccountLocked(c.Context(), h.cache, user.ID) {
		return nil, c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}
	return user, nil
}

func (h *WebAuthnHandler) loadLoginAttempt(c fiber.Ctx, token string) (*model.User, webauthn.SessionData, error) {
	cached, err := h.cache.Get(c.Context(), "webauthn_login:"+token)
	if err != nil || cached == "" {
		return nil, webauthn.SessionData{}, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
	}
	userID, sessionJSON, found := splitCachedLoginSession(cached)
	if !found {
		return nil, webauthn.SessionData{}, c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "invalid cached data"})
	}
	if handlercommon.IsAccountLocked(c.Context(), h.cache, userID) {
		return nil, webauthn.SessionData{}, c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}
	session, err := loadSessionData(sessionJSON)
	if err != nil {
		return nil, webauthn.SessionData{}, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session"})
	}
	waUser, err := h.loadWebAuthnUser(c, userID)
	if err != nil {
		return nil, webauthn.SessionData{}, c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return waUser.User, session, nil
}

func (h *WebAuthnHandler) validateLoginResponse(
	c fiber.Ctx,
	userID string,
	session webauthn.SessionData,
) (*webauthn.Credential, error) {
	waUser, err := h.loadWebAuthnUser(c, userID)
	if err != nil {
		return nil, c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	parsedResponse, err := loadCredentialRequestResponse(c.Body())
	if err != nil {
		return nil, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid credential response"})
	}
	wa, err := h.getWebAuthn()
	if err != nil {
		return nil, c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}
	return wa.ValidateLogin(waUser, session, parsedResponse)
}

func (h *WebAuthnHandler) handleLoginFailure(c fiber.Ctx, userID, token string) error {
	handlercommon.RecordFailedAuthAttempt(c.Context(), h.cache, userID)
	failKey := "webauthn_fail:" + token
	count, _ := h.cache.Incr(c.Context(), failKey, webAuthnFailTTL)
	if count >= 5 {
		_ = h.cache.Del(c.Context(), "webauthn_login:"+token)
	}
	h.writeFailedLoginAudit(c, userID, model.AuditLoginFailed, "user", "webauthn")
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication failed"})
}

func (h *WebAuthnHandler) completeLoginSuccess(c fiber.Ctx, token, userID string, credentialID []byte, signCount uint32) {
	_ = h.creds.UpdateSignCount(c.Context(), credentialID, signCount)
	_ = h.cache.Del(c.Context(), "webauthn_login:"+token)
	handlercommon.ClearFailedAuthAttempts(c.Context(), h.cache, userID)
	h.writeSuccessfulLoginAudit(c, userID)
}

func (h *WebAuthnHandler) loadMFAAttempt(c fiber.Ctx, mfaToken string) (string, *model.User, webauthn.SessionData, string, error) {
	userID, err := h.cache.Get(c.Context(), "mfa_pending:"+mfaToken)
	if err != nil || userID == "" {
		return "", nil, webauthn.SessionData{}, "", c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired mfa_token"})
	}
	failKey := "mfa_fail:" + userID
	failCount, _ := h.cache.GetInt(c.Context(), failKey)
	if failCount >= 5 {
		return "", nil, webauthn.SessionData{}, "", c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too_many_attempts"})
	}
	sessionJSON, err := h.cache.Get(c.Context(), "webauthn_auth:"+mfaToken)
	if err != nil || sessionJSON == "" {
		return "", nil, webauthn.SessionData{}, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no pending authentication"})
	}
	session, err := loadSessionData(sessionJSON)
	if err != nil {
		return "", nil, webauthn.SessionData{}, "", c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session"})
	}
	waUser, err := h.loadWebAuthnUser(c, userID)
	if err != nil {
		return "", nil, webauthn.SessionData{}, "", c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	return userID, waUser.User, session, failKey, nil
}

func (h *WebAuthnHandler) handleMFAFailure(c fiber.Ctx, userID, failKey string) error {
	_, _ = h.cache.Incr(c.Context(), failKey, mfaFailureTTL)
	handlercommon.RecordFailedAuthAttempt(c.Context(), h.cache, userID)
	h.writeFailedLoginAudit(c, userID, model.AuditMFAVerify, "mfa", "webauthn")
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication failed"})
}

func (h *WebAuthnHandler) completeMFASuccess(c fiber.Ctx, mfaToken, userID, failKey string, credentialID []byte, signCount uint32) time.Duration {
	_ = h.cache.Del(c.Context(), failKey)
	handlercommon.ClearFailedAuthAttempts(c.Context(), h.cache, userID)
	_ = h.creds.UpdateSignCount(c.Context(), credentialID, signCount)
	_ = h.cache.Del(c.Context(), "mfa_pending:"+mfaToken)
	_ = h.cache.Del(c.Context(), "webauthn_auth:"+mfaToken)
	_ = h.cache.Del(c.Context(), "mfa_pending_ctx:"+mfaToken)
	rmKey := "mfa_pending_rm:" + mfaToken
	rmVal, _ := h.cache.Get(c.Context(), rmKey)
	_ = h.cache.Del(c.Context(), rmKey)
	h.writeSuccessfulLoginAudit(c, userID)
	return rememberMeRefreshTTL(rmVal, h.tokenConfig)
}
