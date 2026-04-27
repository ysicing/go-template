package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

const (
	defaultWebAuthnCredentialName = "Passkey"
	webAuthnSessionTTL            = 5 * time.Minute
	webAuthnFailTTL               = 5 * time.Minute
)

func loadSessionData(raw string) (webauthn.SessionData, error) {
	var session webauthn.SessionData
	err := json.Unmarshal([]byte(raw), &session)
	return session, err
}

func loadCredentialCreationResponse(body []byte) (*protocol.ParsedCredentialCreationData, error) {
	return protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body))
}

func loadCredentialRequestResponse(body []byte) (*protocol.ParsedCredentialAssertionData, error) {
	return protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
}

func webAuthnTokenFromRequest(c fiber.Ctx, header, query, message string) (string, error) {
	token := c.Get(header)
	if token == "" {
		token = c.Query(query)
	}
	if token == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, message)
	}
	return token, nil
}

func normalizeWebAuthnCredentialName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return defaultWebAuthnCredentialName
	}
	return name
}

func splitCachedLoginSession(raw string) (string, string, bool) {
	return strings.Cut(raw, "|")
}

func rememberMeRefreshTTL(rememberMeValue string, tokenCfg TokenConfig) time.Duration {
	if rememberMeValue == "1" {
		return tokenCfg.RememberMeTTL
	}
	return tokenCfg.RefreshTTL
}

func (h *WebAuthnHandler) saveSessionData(ctx context.Context, key string, session *webauthn.SessionData) {
	sessionJSON, _ := json.Marshal(session)
	_ = h.cache.Set(ctx, key, string(sessionJSON), webAuthnSessionTTL)
}

func (h *WebAuthnHandler) issueBrowserSession(c fiber.Ctx, user *model.User, refreshTTL time.Duration) error {
	ip, ua := GetRealIPAndUA(c)
	issuedSession, err := h.sessions.IssueBrowserSession(c.Context(), service.SessionRequest{
		User:       user,
		IP:         ip,
		UserAgent:  ua,
		RefreshTTL: refreshTTL,
	})
	if err != nil {
		return err
	}
	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, refreshTTL)
	return nil
}

func (h *WebAuthnHandler) writeSuccessfulLoginAudit(c fiber.Ctx, userID string) {
	loginIP, loginUA := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditLogin, Resource: "user", ResourceID: userID,
		IP: loginIP, UserAgent: loginUA, Status: "success", Detail: "webauthn",
	})
}

func (h *WebAuthnHandler) writeFailedLoginAudit(c fiber.Ctx, userID, action, resource, detail string) {
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: action, Resource: resource, ResourceID: userID,
		IP: GetRealIP(c), UserAgent: c.Get("User-Agent"), Status: "failure", Detail: detail,
	})
}
