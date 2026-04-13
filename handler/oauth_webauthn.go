package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

type oauthWebAuthnCredStore interface {
	ListByUserID(ctx context.Context, userID string) ([]model.WebAuthnCredential, error)
	UpdateSignCount(ctx context.Context, credentialID []byte, signCount uint32) error
}

type oauthWebAuthnManager interface {
	BeginLogin(user *store.WebAuthnUser) (*protocol.CredentialAssertion, *webauthn.SessionData, error)
	FinishLogin(user *store.WebAuthnUser, session webauthn.SessionData, body []byte) (*webauthn.Credential, error)
}

type defaultOAuthWebAuthnManager struct {
	settings oauthSettingStore
}

func (m defaultOAuthWebAuthnManager) BeginLogin(user *store.WebAuthnUser) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	wa, err := m.build()
	if err != nil {
		return nil, nil, err
	}
	return wa.BeginLogin(user)
}

func (m defaultOAuthWebAuthnManager) FinishLogin(user *store.WebAuthnUser, session webauthn.SessionData, body []byte) (*webauthn.Credential, error) {
	wa, err := m.build()
	if err != nil {
		return nil, err
	}
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return wa.ValidateLogin(user, session, parsedResponse)
}

func (m defaultOAuthWebAuthnManager) build() (*webauthn.WebAuthn, error) {
	if m.settings == nil {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "webauthn not configured")
	}

	rpID := strings.TrimSpace(m.settings.Get(store.SettingWebAuthnRPID, ""))
	if rpID == "" {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "webauthn not configured")
	}

	rpDisplay := m.settings.Get(store.SettingWebAuthnRPDisplay, "ID Service")
	rpOrigins := m.settings.Get(store.SettingWebAuthnRPOrigins, "")
	origins := make([]string, 0, 4)
	for origin := range strings.SplitSeq(rpOrigins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	return webauthn.New(&webauthn.Config{
		RPDisplayName: rpDisplay,
		RPID:          rpID,
		RPOrigins:     origins,
	})
}

func (h *OAuthHandler) loadSocialLinkWebAuthnUser(c fiber.Ctx, userID string) (*store.WebAuthnUser, error) {
	if h.webAuthnCreds == nil {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "webauthn not configured")
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return nil, err
	}
	creds, err := h.webAuthnCreds.ListByUserID(c.Context(), userID)
	if err != nil {
		return nil, err
	}
	return &store.WebAuthnUser{User: user, Creds: creds}, nil
}

// SocialLinkWebAuthnBegin handles POST /api/auth/social/confirm-link/webauthn/begin.
func (h *OAuthHandler) SocialLinkWebAuthnBegin(c fiber.Ctx) error {
	var req struct {
		LinkToken string `json:"link_token"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.LinkToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "link_token is required"})
	}
	if h.webAuthn == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	pending, err := h.loadPendingSocialLink(c.Context(), req.LinkToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired link token"})
	}
	if isAccountLocked(c.Context(), h.cache, pending.UserID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}

	waUser, err := h.loadSocialLinkWebAuthnUser(c, pending.UserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}
	if len(waUser.Creds) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "webauthn_not_enabled",
			"hint":  "WebAuthn is not enabled for this account. Please use password or TOTP first.",
		})
	}

	options, session, err := h.webAuthn.BeginLogin(waUser)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	sessionJSON, _ := json.Marshal(session)
	if err := h.cache.Set(c.Context(), socialLinkWebAuthnKey(req.LinkToken), string(sessionJSON), 5*time.Minute); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to persist webauthn session"})
	}

	return c.JSON(fiber.Map{"publicKey": options.Response})
}

// SocialLinkWebAuthnFinish handles POST /api/auth/social/confirm-link/webauthn/finish.
func (h *OAuthHandler) SocialLinkWebAuthnFinish(c fiber.Ctx) error {
	linkToken := c.Query("link_token")
	if linkToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "link_token query param is required"})
	}
	if h.webAuthn == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "webauthn not configured"})
	}

	pending, err := h.loadPendingSocialLink(c.Context(), linkToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired link token"})
	}
	if isAccountLocked(c.Context(), h.cache, pending.UserID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}

	sessionJSON, err := h.cache.Get(c.Context(), socialLinkWebAuthnKey(linkToken))
	if err != nil || sessionJSON == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no pending authentication"})
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session"})
	}

	waUser, err := h.loadSocialLinkWebAuthnUser(c, pending.UserID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	credential, err := h.webAuthn.FinishLogin(waUser, session, c.Body())
	if err != nil {
		recordFailedAuthAttempt(c.Context(), h.cache, pending.UserID)
		_ = recordAuditFromFiber(c, h.audit, AuditEvent{
			UserID:   pending.UserID,
			Action:   model.AuditSocialAccountLink,
			Resource: "social_account",
			Status:   "failed",
			Detail:   "social link verification failed",
			Metadata: map[string]string{
				"reason": "invalid_webauthn",
			},
		})
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication failed"})
	}

	if h.webAuthnCreds != nil {
		_ = h.webAuthnCreds.UpdateSignCount(c.Context(), credential.ID, credential.Authenticator.SignCount)
	}

	return h.completeSocialLink(c, waUser.User, pending, "webauthn",
		socialLinkPendingKey(linkToken),
		socialLinkWebAuthnKey(linkToken),
	)
}
