package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/crypto/bcrypt"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

type mfaUserStore interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
}

type mfaStore interface {
	GetByUserID(ctx context.Context, userID string) (*model.MFAConfig, error)
	Upsert(ctx context.Context, cfg *model.MFAConfig) error
	Delete(ctx context.Context, userID string) error
}

type oidcAuthCompleter interface {
	CompleteAuthRequest(ctx context.Context, id, userID string) error
	AssignAuthRequestUser(ctx context.Context, id, userID string) error
	AuthRequestRequiresConsent(ctx context.Context, id string) bool
	AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error)
}

// MFADeps aggregates dependencies required by MFAHandler.
type MFADeps struct {
	Users         mfaUserStore
	MFA           mfaStore
	Audit         *store.AuditLogStore
	RefreshTokens refreshTokenCreator
	Sessions      *service.SessionService
	Cache         store.Cache
	OIDC          oidcAuthCompleter
	Clients       *store.OAuthClientStore
	ConsentGrants *store.OAuthConsentGrantStore
	TokenConfig   TokenConfig
}

// MFAHandler handles MFA endpoints.
type MFAHandler struct {
	users         mfaUserStore
	mfa           mfaStore
	audit         *store.AuditLogStore
	sessions      *service.SessionService
	cache         store.Cache
	oidc          oidcAuthCompleter
	clients       *store.OAuthClientStore
	consentGrants *store.OAuthConsentGrantStore
	tokenConfig   TokenConfig
}

const (
	mfaVerifyThreshold = 5
	mfaVerifyWindow    = 5 * time.Minute
)

// NewMFAHandler creates a MFAHandler.
func NewMFAHandler(deps MFADeps) *MFAHandler {
	sessions := deps.Sessions
	if sessions == nil {
		sessions = service.NewSessionService(deps.RefreshTokens, deps.TokenConfig.ToServiceConfig())
	}

	return &MFAHandler{
		users:         deps.Users,
		mfa:           deps.MFA,
		audit:         deps.Audit,
		sessions:      sessions,
		cache:         deps.Cache,
		oidc:          deps.OIDC,
		clients:       deps.Clients,
		consentGrants: deps.ConsentGrants,
		tokenConfig:   deps.TokenConfig,
	}
}

func mfaVerifyFailKey(token string) string { return "mfa_verify_fail:" + token }
func mfaVerifyLockKey(token string) string { return "mfa_verify_lock:" + token }
func mfaConsumedKey(token string) string   { return "mfa_consumed:" + token }

func (h *MFAHandler) isMFAVerifyLocked(ctx context.Context, token string) bool {
	val, err := h.cache.Get(ctx, mfaVerifyLockKey(token))
	return err == nil && val != ""
}

func (h *MFAHandler) recordFailedMFAVerify(ctx context.Context, token string) bool {
	key := mfaVerifyFailKey(token)
	count, _ := h.cache.Incr(ctx, key, mfaVerifyWindow)
	if count >= int64(mfaVerifyThreshold) {
		_ = h.cache.Set(ctx, mfaVerifyLockKey(token), "1", mfaVerifyWindow)
		return true
	}
	return false
}

func (h *MFAHandler) clearMFAVerifyFailures(ctx context.Context, token string) {
	_ = h.cache.Del(ctx, mfaVerifyFailKey(token))
	_ = h.cache.Del(ctx, mfaVerifyLockKey(token))
}

// Status handles GET /api/mfa/status.
func (h *MFAHandler) Status(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	cfg, err := h.mfa.GetByUserID(c.Context(), userID)
	if err != nil {
		return c.JSON(fiber.Map{"totp_enabled": false})
	}
	return c.JSON(fiber.Map{"totp_enabled": cfg.TOTPEnabled})
}

// TOTPSetup handles POST /api/mfa/totp/setup.
func (h *MFAHandler) TOTPSetup(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      h.tokenConfig.Issuer,
		AccountName: user.Email,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate TOTP secret"})
	}

	// Store secret temporarily in cache until confirmed.
	_ = h.cache.Set(c.Context(), "totp_setup:"+userID, key.Secret(), 10*time.Minute)

	return c.JSON(fiber.Map{
		"secret": key.Secret(),
		"url":    key.URL(),
	})
}

// TOTPEnable handles POST /api/mfa/totp/enable.
func (h *MFAHandler) TOTPEnable(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	var req struct {
		Code string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code is required"})
	}

	secret, err := h.cache.Get(c.Context(), "totp_setup:"+userID)
	if err != nil || secret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no pending TOTP setup, call /api/mfa/totp/setup first"})
	}

	if !totp.Validate(req.Code, secret) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid TOTP code"})
	}

	// Generate backup codes.
	backupCodes := generateBackupCodes(10)
	hashedCodes := hashBackupCodes(backupCodes)
	codesJSON, err := json.Marshal(hashedCodes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encode backup codes"})
	}

	cfg := &model.MFAConfig{
		UserID:      userID,
		TOTPSecret:  secret,
		TOTPEnabled: true,
		BackupCodes: string(codesJSON),
	}
	if err := h.mfa.Upsert(c.Context(), cfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to enable TOTP"})
	}

	_ = h.cache.Del(c.Context(), "totp_setup:"+userID)

	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditMFAEnable, Resource: "mfa",
		IP: ip, UserAgent: ua, Status: "success",
	})

	return c.JSON(fiber.Map{
		"message":      "TOTP enabled",
		"backup_codes": backupCodes,
	})
}

// TOTPDisable handles POST /api/mfa/totp/disable.
func (h *MFAHandler) TOTPDisable(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Rate-limit MFA disable attempts using the same lockout mechanism as login.
	if isAccountLocked(c.Context(), h.cache, userID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many attempts, try again later"})
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	// Social login users (no password) can verify with TOTP code instead.
	if user.PasswordHash == "" {
		if req.Code == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "totp code is required"})
		}
		cfg, mfaErr := h.mfa.GetByUserID(c.Context(), userID)
		if mfaErr != nil || !cfg.TOTPEnabled {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "TOTP not enabled"})
		}
		if !totp.Validate(req.Code, cfg.TOTPSecret) {
			recordFailedAuthAttempt(c.Context(), h.cache, userID)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid TOTP code"})
		}
	} else {
		if req.Password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password is required"})
		}
		if !user.CheckPassword(req.Password) {
			recordFailedAuthAttempt(c.Context(), h.cache, userID)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid password"})
		}
	}

	if err := h.mfa.Delete(c.Context(), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to disable TOTP"})
	}

	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditMFADisable, Resource: "mfa",
		IP: ip, UserAgent: ua, Status: "success",
	})

	return c.JSON(fiber.Map{"message": "TOTP disabled"})
}

// RegenerateBackupCodes handles POST /api/mfa/backup-codes/regenerate.
func (h *MFAHandler) RegenerateBackupCodes(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)

	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Rate-limit regenerate attempts
	if isAccountLocked(c.Context(), h.cache, userID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many attempts, try again later"})
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	cfg, err := h.mfa.GetByUserID(c.Context(), userID)
	if err != nil || !cfg.TOTPEnabled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "TOTP not enabled"})
	}

	// Social login users (no password) must verify with TOTP code
	if user.PasswordHash == "" {
		if req.Code == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "totp code is required"})
		}
		if !totp.Validate(req.Code, cfg.TOTPSecret) {
			recordFailedAuthAttempt(c.Context(), h.cache, userID)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid TOTP code"})
		}
	} else {
		// Local users must verify with password
		if req.Password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password is required"})
		}
		if !user.CheckPassword(req.Password) {
			recordFailedAuthAttempt(c.Context(), h.cache, userID)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid password"})
		}
	}

	backupCodes := generateBackupCodes(10)
	hashedCodes := hashBackupCodes(backupCodes)
	codesJSON, err := json.Marshal(hashedCodes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to encode backup codes"})
	}
	cfg.BackupCodes = string(codesJSON)

	if err := h.mfa.Upsert(c.Context(), cfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to regenerate backup codes"})
	}

	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditMFABackupRegenerate, Resource: "mfa",
		IP: ip, UserAgent: ua, Status: "success",
	})

	return c.JSON(fiber.Map{"backup_codes": backupCodes})
}

// Verify handles POST /api/auth/mfa/verify.
func (h *MFAHandler) Verify(c fiber.Ctx) error {
	var req struct {
		MFAToken   string `json:"mfa_token"`
		Code       string `json:"code"`
		BackupCode string `json:"backup_code"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.MFAToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mfa_token is required"})
	}

	// Verify the pending token exists before consuming it, to avoid permanently
	// burning the token when the pending key has already expired.
	userID, err := h.cache.Get(c.Context(), "mfa_pending:"+req.MFAToken)
	if err != nil || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired mfa_token"})
	}
	// Atomically consume the MFA token to prevent TOCTOU double-use
	consumed, err := h.cache.SetNX(c.Context(), mfaConsumedKey(req.MFAToken), "1", 5*time.Minute)
	if err != nil || !consumed {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired mfa_token"})
	}
	if h.isMFAVerifyLocked(c.Context(), req.MFAToken) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many MFA attempts, please log in again"})
	}

	cfg, err := h.mfa.GetByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "MFA config not found"})
	}

	verified := false

	if req.Code != "" {
		// TOTP verification.
		if totp.Validate(req.Code, cfg.TOTPSecret) {
			verified = true
		}
	} else if req.BackupCode != "" {
		// Backup code verification.
		verified = h.verifyAndConsumeBackupCode(c.Context(), cfg, req.BackupCode)
	}

	if !verified {
		_ = h.cache.Del(c.Context(), mfaConsumedKey(req.MFAToken))
		locked := h.recordFailedMFAVerify(c.Context(), req.MFAToken)
		ip, ua := GetRealIPAndUA(c)
		_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
			UserID: userID, Action: model.AuditMFAVerify, Resource: "mfa",
			IP: ip, UserAgent: ua, Status: "failure",
		})
		if locked {
			_ = h.cache.Del(c.Context(), "mfa_pending:"+req.MFAToken)
			_ = h.cache.Del(c.Context(), "webauthn_auth:"+req.MFAToken)
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many MFA attempts, please log in again"})
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid MFA code"})
	}
	h.clearMFAVerifyFailures(c.Context(), req.MFAToken)

	ctxKey := "mfa_pending_ctx:" + req.MFAToken
	pendingCtx, _ := h.cache.Get(c.Context(), ctxKey)

	// Clean up MFA pending tokens after successful verification.
	_ = h.cache.Del(c.Context(), "mfa_pending:"+req.MFAToken)
	_ = h.cache.Del(c.Context(), ctxKey)

	// Determine refresh TTL based on remember_me flag stored during login.
	rmKey := "mfa_pending_rm:" + req.MFAToken
	rmVal, _ := h.cache.Get(c.Context(), rmKey)
	_ = h.cache.Del(c.Context(), rmKey)
	refreshTTL := h.tokenConfig.RefreshTTL
	if rmVal == "1" {
		refreshTTL = h.tokenConfig.RememberMeTTL
	}

	if strings.HasPrefix(pendingCtx, "oidc:") {
		authReqID := strings.TrimPrefix(pendingCtx, "oidc:")
		if authReqID == "" || h.oidc == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid oidc login context"})
		}
		shouldPrompt := h.oidc.AuthRequestRequiresConsent(c.Context(), authReqID)
		if !shouldPrompt && h.clients != nil {
			var err error
			shouldPrompt, err = shouldPromptOIDCConsent(c.Context(), h.oidc, h.clients, h.consentGrants, authReqID, userID)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
			}
		}
		if shouldPrompt {
			if err := h.oidc.AssignAuthRequestUser(c.Context(), authReqID, userID); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
			}
			return c.JSON(fiber.Map{"redirect": "/consent?id=" + authReqID})
		}
		if err := h.oidc.CompleteAuthRequest(c.Context(), authReqID, userID); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
		}
		return c.JSON(fiber.Map{"redirect": "/authorize/callback?id=" + authReqID})
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditLogin, Resource: "user", ResourceID: userID,
		IP: ip, UserAgent: ua, Status: "success", Detail: "local",
	})

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

func (h *MFAHandler) verifyAndConsumeBackupCode(ctx context.Context, cfg *model.MFAConfig, code string) bool {
	var hashes []string
	if err := json.Unmarshal([]byte(cfg.BackupCodes), &hashes); err != nil {
		return false
	}

	for i, hash := range hashes {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			// Remove used code.
			hashes = append(hashes[:i], hashes[i+1:]...)
			codesJSON, err := json.Marshal(hashes)
			if err != nil {
				return false
			}
			cfg.BackupCodes = string(codesJSON)
			if err := h.mfa.Upsert(ctx, cfg); err != nil {
				return false
			}
			return true
		}
	}
	return false
}

func generateBackupCodes(n int) []string {
	codes := make([]string, n)
	for i := range codes {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		codes[i] = fmt.Sprintf("%016x", b)
	}
	return codes
}

func hashBackupCodes(codes []string) []string {
	hashed := make([]string, len(codes))
	for i, code := range codes {
		h, _ := bcrypt.GenerateFromPassword([]byte(code), 12)
		hashed[i] = string(h)
	}
	return hashed
}
