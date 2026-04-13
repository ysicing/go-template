package handler

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"
)

const (
	mfaFailureTTL = 5 * time.Minute // TTL for MFA failure counter
)

// WebAuthnDeps aggregates dependencies required by WebAuthnHandler.
type WebAuthnDeps struct {
	Settings      *store.SettingStore
	Users         *store.UserStore
	Creds         *store.WebAuthnStore
	MFA           *store.MFAStore
	Audit         *store.AuditLogStore
	RefreshTokens *store.APIRefreshTokenStore
	Sessions      *service.SessionService
	Cache         store.Cache
	TokenConfig   TokenConfig
}

// WebAuthnHandler handles WebAuthn/Passkey endpoints.
type WebAuthnHandler struct {
	settings    *store.SettingStore
	users       *store.UserStore
	creds       *store.WebAuthnStore
	mfa         *store.MFAStore
	audit       *store.AuditLogStore
	sessions    *service.SessionService
	cache       store.Cache
	tokenConfig TokenConfig

	mu        sync.RWMutex
	cachedWA  *webauthn.WebAuthn
	cachedAt  time.Time
	cachedCfg string
}

// NewWebAuthnHandler creates a WebAuthnHandler.
func NewWebAuthnHandler(deps WebAuthnDeps) *WebAuthnHandler {
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

	return &WebAuthnHandler{
		settings:    deps.Settings,
		users:       deps.Users,
		creds:       deps.Creds,
		mfa:         deps.MFA,
		audit:       deps.Audit,
		sessions:    sessions,
		cache:       deps.Cache,
		tokenConfig: deps.TokenConfig,
	}
}

// getWebAuthn returns a cached *webauthn.WebAuthn instance, rebuilding when settings change.
func (h *WebAuthnHandler) getWebAuthn() (*webauthn.WebAuthn, error) {
	rpID := h.settings.Get(store.SettingWebAuthnRPID, "")
	rpDisplay := h.settings.Get(store.SettingWebAuthnRPDisplay, "ID Service")
	rpOrigins := h.settings.Get(store.SettingWebAuthnRPOrigins, "")
	cfgKey := rpID + "|" + rpDisplay + "|" + rpOrigins

	h.mu.RLock()
	if h.cachedWA != nil && h.cachedCfg == cfgKey && time.Since(h.cachedAt) < 30*time.Second {
		cached := h.cachedWA
		h.mu.RUnlock()
		return cached, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cachedWA != nil && h.cachedCfg == cfgKey && time.Since(h.cachedAt) < 30*time.Second {
		return h.cachedWA, nil
	}

	if rpID == "" {
		h.cachedWA = nil
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "webauthn not configured")
	}

	var origins []string
	for o := range strings.SplitSeq(rpOrigins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}

	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: rpDisplay,
		RPID:          rpID,
		RPOrigins:     origins,
	})
	if err != nil {
		return nil, err
	}

	h.cachedWA = wa
	h.cachedAt = time.Now()
	h.cachedCfg = cfgKey
	return wa, nil
}

func (h *WebAuthnHandler) loadWebAuthnUser(c fiber.Ctx, userID string) (*store.WebAuthnUser, error) {
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return nil, err
	}
	creds, _ := h.creds.ListByUserID(c.Context(), userID)
	return &store.WebAuthnUser{User: user, Creds: creds}, nil
}

// ListCredentials handles GET /api/mfa/webauthn/credentials.
func (h *WebAuthnHandler) ListCredentials(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	creds, err := h.creds.ListByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list credentials"})
	}
	type credResp struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"created_at"`
	}
	resp := make([]credResp, len(creds))
	for i, cr := range creds {
		resp[i] = credResp{ID: cr.ID, Name: cr.Name, CreatedAt: cr.CreatedAt}
	}
	return c.JSON(fiber.Map{"credentials": resp})
}

// RegisterBegin handles POST /api/mfa/webauthn/register/begin.
func (h *WebAuthnHandler) RegisterBegin(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	waUser, err := h.loadWebAuthnUser(c, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	wa, err := h.getWebAuthn()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	options, session, err := wa.BeginRegistration(waUser)
	if err != nil {
		logger.L.Error().Err(err).Msg("webauthn begin registration")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to begin registration"})
	}

	sessionJSON, _ := json.Marshal(session)
	_ = h.cache.Set(c.Context(), "webauthn_reg:"+userID, string(sessionJSON), 5*time.Minute)

	return c.JSON(options)
}

// RegisterFinish handles POST /api/mfa/webauthn/register/finish.
func (h *WebAuthnHandler) RegisterFinish(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	waUser, err := h.loadWebAuthnUser(c, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	sessionJSON, err := h.cache.Get(c.Context(), "webauthn_reg:"+userID)
	if err != nil || sessionJSON == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no pending registration"})
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session"})
	}

	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(c.Body()))
	if err != nil {
		logger.L.Error().Err(err).Msg("webauthn parse credential creation response")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid credential response"})
	}

	wa2, err := h.getWebAuthn()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	credential, err := wa2.CreateCredential(waUser, session, parsedResponse)
	if err != nil {
		logger.L.Error().Err(err).Msg("webauthn create credential")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "failed to create credential"})
	}

	_ = h.cache.Del(c.Context(), "webauthn_reg:"+userID)

	// Request body is consumed by the WebAuthn response, so use query param only.
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		name = "Passkey"
	}

	transportsJSON, _ := json.Marshal(credential.Transport)
	dbCred := &model.WebAuthnCredential{
		UserID:         userID,
		Name:           name,
		CredentialID:   credential.ID,
		PublicKey:      credential.PublicKey,
		AAGUID:         credential.Authenticator.AAGUID,
		SignCount:      credential.Authenticator.SignCount,
		BackupEligible: credential.Flags.BackupEligible,
		BackupState:    credential.Flags.BackupState,
		Transport:      string(transportsJSON),
	}
	if err := h.creds.Create(c.Context(), dbCred); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save credential"})
	}

	waAddIP, waAddUA := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditWebAuthnAdd, Resource: "webauthn", ResourceID: dbCred.ID,
		IP: waAddIP, UserAgent: waAddUA, Status: "success",
	})

	return c.JSON(fiber.Map{"message": "credential registered", "id": dbCred.ID})
}

// DeleteCredential handles DELETE /api/mfa/webauthn/credentials/:id.
func (h *WebAuthnHandler) DeleteCredential(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	credID := c.Params("id")

	if err := h.creds.Delete(c.Context(), credID, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete credential"})
	}

	waDelIP, waDelUA := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditWebAuthnRemove, Resource: "webauthn", ResourceID: credID,
		IP: waDelIP, UserAgent: waDelUA, Status: "success",
	})

	return c.JSON(fiber.Map{"message": "credential deleted"})
}

// LoginBegin handles POST /api/auth/webauthn/begin (passwordless login).
func (h *WebAuthnHandler) LoginBegin(c fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username is required"})
	}

	user, err := h.users.GetByUsername(c.Context(), req.Username)
	if err != nil {
		user, err = h.users.GetByEmail(c.Context(), req.Username)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
		}
	}

	if isAccountLocked(c.Context(), h.cache, user.ID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}

	waUser, err := h.loadWebAuthnUser(c, user.ID)
	if err != nil || len(waUser.Creds) == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	waLogin, err := h.getWebAuthn()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	options, session, err := waLogin.BeginLogin(waUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to begin authentication"})
	}

	token := store.GenerateRandomToken()
	sessionJSON, _ := json.Marshal(session)
	_ = h.cache.Set(c.Context(), "webauthn_login:"+token, user.ID+"|"+string(sessionJSON), 5*time.Minute)

	return c.JSON(fiber.Map{
		"publicKey":      options.Response,
		"webauthn_token": token,
	})
}

// LoginFinish handles POST /api/auth/webauthn/finish (passwordless login).
func (h *WebAuthnHandler) LoginFinish(c fiber.Ctx) error {
	token := c.Query("webauthn_token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "webauthn_token is required"})
	}

	cached, err := h.cache.Get(c.Context(), "webauthn_login:"+token)
	if err != nil || cached == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
	}

	// Split "userID|sessionJSON"
	userID, sessionJSON, found := strings.Cut(cached, "|")
	if !found {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "invalid cached data"})
	}

	if isAccountLocked(c.Context(), h.cache, userID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session"})
	}

	waUser, err := h.loadWebAuthnUser(c, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(c.Body()))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid credential response"})
	}

	waValidate, err := h.getWebAuthn()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	credential, err := waValidate.ValidateLogin(waUser, session, parsedResponse)
	if err != nil {
		recordFailedAuthAttempt(c.Context(), h.cache, userID)
		// Rate-limit WebAuthn login failures per token.
		failKey := "webauthn_fail:" + token
		count, _ := h.cache.Incr(c.Context(), failKey, 5*time.Minute)
		if count >= 5 {
			_ = h.cache.Del(c.Context(), "webauthn_login:"+token)
		}
		_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
			UserID: userID, Action: model.AuditLoginFailed, Resource: "user", ResourceID: userID,
			IP: GetRealIP(c), UserAgent: c.Get("User-Agent"), Status: "failure", Detail: "webauthn",
		})
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication failed"})
	}

	_ = h.creds.UpdateSignCount(c.Context(), credential.ID, credential.Authenticator.SignCount)
	_ = h.cache.Del(c.Context(), "webauthn_login:"+token)
	clearFailedAuthAttempts(c.Context(), h.cache, userID)

	user := waUser.User

	waLoginIP, waLoginUA := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditLogin, Resource: "user", ResourceID: userID,
		IP: waLoginIP, UserAgent: waLoginUA, Status: "success", Detail: "webauthn",
	})

	ip, ua := GetRealIPAndUA(c)
	issuedSession, err := h.sessions.IssueBrowserSession(c.Context(), service.SessionRequest{
		User:       user,
		IP:         ip,
		UserAgent:  ua,
		RefreshTTL: h.tokenConfig.RefreshTTL,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}

	// Set tokens in cookies for web clients
	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, h.tokenConfig.RefreshTTL)

	return c.JSON(fiber.Map{
		"user": user,
	})
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

	waAuth, err := h.getWebAuthn()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	options, session, err := waAuth.BeginLogin(waUser)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to begin authentication"})
	}

	sessionJSON, _ := json.Marshal(session)
	_ = h.cache.Set(c.Context(), "webauthn_auth:"+req.MFAToken, string(sessionJSON), 5*time.Minute)

	return c.JSON(options)
}

// AuthFinish handles POST /api/auth/mfa/webauthn/finish.
func (h *WebAuthnHandler) AuthFinish(c fiber.Ctx) error {
	mfaToken := c.Query("mfa_token")
	if mfaToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mfa_token query param is required"})
	}

	userID, err := h.cache.Get(c.Context(), "mfa_pending:"+mfaToken)
	if err != nil || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired mfa_token"})
	}

	// Check for account lockout due to too many failed MFA attempts
	failKey := "mfa_fail:" + userID
	failCount, _ := h.cache.GetInt(c.Context(), failKey) // Read current count without incrementing
	if failCount >= 5 {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too_many_attempts"})
	}

	sessionJSON, err := h.cache.Get(c.Context(), "webauthn_auth:"+mfaToken)
	if err != nil || sessionJSON == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no pending authentication"})
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session"})
	}

	waUser, err := h.loadWebAuthnUser(c, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(c.Body()))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid credential response"})
	}

	waFinish, err := h.getWebAuthn()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	credential, err := waFinish.ValidateLogin(waUser, session, parsedResponse)
	if err != nil {
		// Increment failure counter on authentication failure
		_, _ = h.cache.Incr(c.Context(), failKey, mfaFailureTTL)
		recordFailedAuthAttempt(c.Context(), h.cache, userID)
		_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
			UserID: userID, Action: model.AuditMFAVerify, Resource: "mfa",
			IP: GetRealIP(c), UserAgent: c.Get("User-Agent"), Status: "failure",
			Detail: "webauthn",
		})
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication failed"})
	}

	// Clear failure counter on success
	_ = h.cache.Del(c.Context(), failKey)
	clearFailedAuthAttempts(c.Context(), h.cache, userID)

	// Update sign count.
	_ = h.creds.UpdateSignCount(c.Context(), credential.ID, credential.Authenticator.SignCount)

	// Clean up.
	_ = h.cache.Del(c.Context(), "mfa_pending:"+mfaToken)
	_ = h.cache.Del(c.Context(), "webauthn_auth:"+mfaToken)
	_ = h.cache.Del(c.Context(), "mfa_pending_ctx:"+mfaToken)

	// Determine refresh TTL based on remember_me flag stored during login.
	rmKey := "mfa_pending_rm:" + mfaToken
	rmVal, _ := h.cache.Get(c.Context(), rmKey)
	_ = h.cache.Del(c.Context(), rmKey)
	refreshTTL := h.tokenConfig.RefreshTTL
	if rmVal == "1" {
		refreshTTL = h.tokenConfig.RememberMeTTL
	}

	user := waUser.User

	waVerifyIP, waVerifyUA := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditLogin, Resource: "user", ResourceID: userID,
		IP: waVerifyIP, UserAgent: waVerifyUA, Status: "success", Detail: "webauthn",
	})

	ip, ua := GetRealIPAndUA(c)
	issuedSession, err := h.sessions.IssueBrowserSession(c.Context(), service.SessionRequest{
		User:       user,
		IP:         ip,
		UserAgent:  ua,
		RefreshTTL: refreshTTL,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}

	// Set tokens in cookies for web clients
	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, refreshTTL)

	return c.JSON(fiber.Map{
		"user": user,
	})
}
