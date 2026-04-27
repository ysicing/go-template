package handler

import (
	"strings"
	"time"

	sessionservice "github.com/ysicing/go-template/internal/service/session"
	"github.com/ysicing/go-template/model"

	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
)

// Verify handles POST /api/auth/mfa/verify.
func (h *MFAHandler) Verify(c fiber.Ctx) error {
	req, userID, pendingCtx, err := h.loadMFAVerifyContext(c)
	if err != nil {
		return finishHandlerError(c, err)
	}

	cfg, err := h.mfa.GetByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "MFA config not found"})
	}
	if err := h.verifyMFACode(c, req, userID, cfg); err != nil {
		return finishHandlerError(c, err)
	}

	refreshTTL := h.consumeMFAVerifyContext(c, req.MFAToken)
	if strings.HasPrefix(pendingCtx, "oidc:") {
		return h.completeOIDCMFAVerify(c, pendingCtx, userID)
	}
	return h.finishBrowserMFAVerify(c, userID, refreshTTL)
}

type mfaVerifyRequest struct {
	MFAToken   string `json:"mfa_token"`
	Code       string `json:"code"`
	BackupCode string `json:"backup_code"`
}

func (h *MFAHandler) loadMFAVerifyContext(c fiber.Ctx) (*mfaVerifyRequest, string, string, error) {
	var req mfaVerifyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return nil, "", "", jsonError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.MFAToken == "" {
		return nil, "", "", jsonError(fiber.StatusBadRequest, "mfa_token is required")
	}

	userID, err := h.cache.Get(c.Context(), "mfa_pending:"+req.MFAToken)
	if err != nil || userID == "" {
		return nil, "", "", jsonError(fiber.StatusUnauthorized, "invalid or expired mfa_token")
	}
	consumed, err := h.cache.SetNX(c.Context(), mfaConsumedKey(req.MFAToken), "1", 5*time.Minute)
	if err != nil || !consumed {
		return nil, "", "", jsonError(fiber.StatusUnauthorized, "invalid or expired mfa_token")
	}
	if h.isMFAVerifyLocked(c.Context(), req.MFAToken) {
		return nil, "", "", jsonError(fiber.StatusTooManyRequests, "too many MFA attempts, please log in again")
	}

	pendingCtx, _ := h.cache.Get(c.Context(), "mfa_pending_ctx:"+req.MFAToken)
	return &req, userID, pendingCtx, nil
}

func (h *MFAHandler) verifyMFACode(c fiber.Ctx, req *mfaVerifyRequest, userID string, cfg *model.MFAConfig) error {
	verified := req.Code != "" && totp.Validate(req.Code, cfg.TOTPSecret)
	if !verified && req.BackupCode != "" {
		verified = h.verifyAndConsumeBackupCode(c.Context(), cfg, req.BackupCode)
	}
	if verified {
		h.clearMFAVerifyFailures(c.Context(), req.MFAToken)
		return nil
	}

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
		return jsonError(fiber.StatusTooManyRequests, "too many MFA attempts, please log in again")
	}
	return jsonError(fiber.StatusUnauthorized, "invalid MFA code")
}

func (h *MFAHandler) consumeMFAVerifyContext(c fiber.Ctx, mfaToken string) time.Duration {
	ctxKey := "mfa_pending_ctx:" + mfaToken
	_ = h.cache.Del(c.Context(), "mfa_pending:"+mfaToken)
	_ = h.cache.Del(c.Context(), ctxKey)

	rmKey := "mfa_pending_rm:" + mfaToken
	rmVal, _ := h.cache.Get(c.Context(), rmKey)
	_ = h.cache.Del(c.Context(), rmKey)
	if rmVal == "1" {
		return h.tokenConfig.RememberMeTTL
	}
	return h.tokenConfig.RefreshTTL
}

func (h *MFAHandler) completeOIDCMFAVerify(c fiber.Ctx, pendingCtx, userID string) error {
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

func (h *MFAHandler) finishBrowserMFAVerify(c fiber.Ctx, userID string, refreshTTL time.Duration) error {
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user not found"})
	}

	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditLogin, Resource: "user", ResourceID: userID,
		IP: ip, UserAgent: ua, Status: "success", Detail: "local",
	})

	issuedSession, err := h.sessions.IssueBrowserSession(c.Context(), sessionservice.SessionRequest{
		User:       user,
		IP:         ip,
		UserAgent:  ua,
		RefreshTTL: refreshTTL,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}

	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, refreshTTL)
	return c.JSON(fiber.Map{"user": NewUserResponse(user)})
}
