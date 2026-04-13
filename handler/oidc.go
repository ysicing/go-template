package handler

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"
)

// OIDCLoginHandler handles the OIDC login submission.
type OIDCLoginHandler struct {
	storage *store.OIDCStorage
	clients *store.OAuthClientStore
	grants  *store.OAuthConsentGrantStore
	mfa     *store.MFAStore
	audit   *store.AuditLogStore
	cache   store.Cache
	auth    *service.AuthService
}

// NewOIDCLoginHandler creates an OIDCLoginHandler.
func NewOIDCLoginHandler(storage *store.OIDCStorage, clients *store.OAuthClientStore, grants *store.OAuthConsentGrantStore, users *store.UserStore, mfa *store.MFAStore, audit *store.AuditLogStore, cache store.Cache) *OIDCLoginHandler {
	return &OIDCLoginHandler{
		storage: storage,
		clients: clients,
		grants:  grants,
		mfa:     mfa,
		audit:   audit,
		cache:   cache,
		auth: service.NewAuthService(service.AuthServiceDeps{
			Users: users,
			Cache: cache,
		}),
	}
}

type oidcLoginRequest struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type oidcConsentRequest struct {
	ID string `json:"id"`
}

func (h *OIDCLoginHandler) shouldPromptConsent(ctx context.Context, authRequestID, userID string) (bool, error) {
	return shouldPromptOIDCConsent(ctx, h.storage, h.clients, h.grants, authRequestID, userID)
}

// LoginSubmit validates credentials and completes the OIDC auth request.
func (h *OIDCLoginHandler) LoginSubmit(c fiber.Ctx) error {
	var req oidcLoginRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).
			JSON(fiber.Map{"error": "invalid request"})
	}
	if req.ID == "" {
		return c.Status(fiber.StatusBadRequest).
			JSON(fiber.Map{"error": "missing auth request id"})
	}

	user, err := h.auth.LoginForAudit(c.Context(), service.LoginInput{
		Identity: req.Username,
		Password: req.Password,
	})
	if errors.Is(err, service.ErrAccountLocked) {
		writeAudit(c.Context(), h.audit, &model.AuditLog{
			UserID: user.ID, Action: model.AuditLoginFailed, Resource: "user", ResourceID: user.ID,
			IP: GetRealIP(c), UserAgent: c.Get("User-Agent"), Status: "failure", Detail: "oidc: account locked",
		})
		return c.Status(fiber.StatusTooManyRequests).
			JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}
	if errors.Is(err, service.ErrInvalidCredentials) {
		if user != nil {
			writeAudit(c.Context(), h.audit, &model.AuditLog{
				UserID: user.ID, Action: model.AuditLoginFailed, Resource: "user", ResourceID: user.ID,
				IP: GetRealIP(c), UserAgent: c.Get("User-Agent"), Status: "failure", Detail: "oidc: invalid password",
			})
		}
		return c.Status(fiber.StatusUnauthorized).
			JSON(fiber.Map{"error": "invalid credentials"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).
			JSON(fiber.Map{"error": "login failed"})
	}

	// Ensure auth request exists and is not expired.
	if _, err := h.storage.AuthRequestByID(c.Context(), req.ID); err != nil {
		return c.Status(fiber.StatusBadRequest).
			JSON(fiber.Map{"error": "auth request not found or expired"})
	}

	mfaCfg, _ := h.mfa.GetByUserID(c.Context(), user.ID)
	if mfaCfg != nil && mfaCfg.TOTPEnabled {
		mfaToken := store.GenerateRandomToken()
		_ = h.cache.Set(c.Context(), "mfa_pending:"+mfaToken, user.ID, 5*time.Minute)
		_ = h.cache.Set(c.Context(), "mfa_pending_ctx:"+mfaToken, "oidc:"+req.ID, 5*time.Minute)
		return c.JSON(fiber.Map{
			"mfa_required": true,
			"mfa_token":    mfaToken,
		})
	}

	redirect := "/authorize/callback?id=" + req.ID
	shouldPrompt, err := h.shouldPromptConsent(c.Context(), req.ID, user.ID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).
			JSON(fiber.Map{"error": "auth request not found or expired"})
	}
	if shouldPrompt {
		if err := h.storage.AssignAuthRequestUser(req.ID, user.ID); err != nil {
			return c.Status(fiber.StatusBadRequest).
				JSON(fiber.Map{"error": "auth request not found or expired"})
		}
		redirect = "/consent?id=" + req.ID
	} else if err := h.storage.CompleteAuthRequest(req.ID, user.ID); err != nil {
		return c.Status(fiber.StatusBadRequest).
			JSON(fiber.Map{"error": "auth request not found or expired"})
	}

	clientID := h.storage.GetAuthRequestClientID(req.ID)
	oidcIP, oidcUA := GetRealIPAndUA(c)
	if err := writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: user.ID, Action: model.AuditLogin, Resource: "user", ResourceID: user.ID,
		ClientID: clientID, IP: oidcIP, UserAgent: oidcUA, Status: "success", Detail: "oidc",
	}); err != nil {
		logger.L.Error().Err(err).Msg("record login event")
	}

	return c.JSON(fiber.Map{
		"redirect": redirect,
	})
}

func (h *OIDCLoginHandler) ConsentContext(c fiber.Ctx) error {
	authReqID := strings.TrimSpace(c.Query("id"))
	if authReqID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing auth request id"})
	}
	authReq, err := h.storage.AuthRequestByID(c.Context(), authReqID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
	}
	required, err := oidcConsentContextRequired(c.Context(), h.storage, h.clients, authReqID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
	}
	if !required {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "consent is not required"})
	}
	client, err := h.clients.GetByClientID(c.Context(), authReq.GetClientID())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "client not found"})
	}

	payload := fiber.Map{
		"client": fiber.Map{
			"id":              client.ClientID,
			"name":            client.Name,
			"require_consent": client.RequireConsent,
		},
		"scopes":           authReq.GetScopes(),
		"requires_consent": true,
	}
	return c.JSON(payload)
}

func (h *OIDCLoginHandler) ConsentApprove(c fiber.Ctx) error {
	var req oidcConsentRequest
	if err := c.Bind().JSON(&req); err != nil || strings.TrimSpace(req.ID) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing auth request id"})
	}
	authReq, err := h.storage.AuthRequestByID(c.Context(), req.ID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
	}
	pending, err := oidcConsentPending(c.Context(), h.storage, h.clients, req.ID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
	}
	if !pending {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "consent is not pending"})
	}
	if h.grants != nil {
		if err := h.grants.Upsert(c.Context(), &model.OAuthConsentGrant{
			UserID:   authReq.GetSubject(),
			ClientID: authReq.GetClientID(),
			Scopes:   strings.Join(authReq.GetScopes(), " "),
		}); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to persist consent grant"})
		}
		if err := writeAudit(c.Context(), h.audit, &model.AuditLog{
			UserID: authReq.GetSubject(), Action: model.AuditOIDCConsentGrantUpsert, Resource: "oauth_consent_grant", ResourceID: authReq.GetSubject() + ":" + authReq.GetClientID(),
			ClientID: authReq.GetClientID(), IP: GetRealIP(c), UserAgent: c.Get("User-Agent"), Status: "success", Detail: "scopes=" + strings.Join(authReq.GetScopes(), " "),
		}); err != nil {
			logger.L.Error().Err(err).Msg("record oidc consent grant upsert")
		}
	}
	if err := h.storage.CompleteAuthRequest(req.ID, authReq.GetSubject()); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
	}

	oidcIP, oidcUA := GetRealIPAndUA(c)
	if err := writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: authReq.GetSubject(), Action: model.AuditOIDCConsentApprove, Resource: "auth_request", ResourceID: req.ID,
		ClientID: authReq.GetClientID(), IP: oidcIP, UserAgent: oidcUA, Status: "success", Detail: "oidc consent approve",
	}); err != nil {
		logger.L.Error().Err(err).Msg("record oidc consent approve")
	}

	return c.JSON(fiber.Map{"redirect": "/authorize/callback?id=" + req.ID})
}

func (h *OIDCLoginHandler) ConsentDeny(c fiber.Ctx) error {
	var req oidcConsentRequest
	if err := c.Bind().JSON(&req); err != nil || strings.TrimSpace(req.ID) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing auth request id"})
	}
	authReq, err := h.storage.AuthRequestByID(c.Context(), req.ID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
	}
	pending, err := oidcConsentPending(c.Context(), h.storage, h.clients, req.ID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
	}
	if !pending {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "consent is not pending"})
	}
	if err := h.storage.DeleteAuthRequest(c.Context(), req.ID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "auth request not found or expired"})
	}

	oidcIP, oidcUA := GetRealIPAndUA(c)
	if err := writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: authReq.GetSubject(), Action: model.AuditOIDCConsentDeny, Resource: "auth_request", ResourceID: req.ID,
		ClientID: authReq.GetClientID(), IP: oidcIP, UserAgent: oidcUA, Status: "success", Detail: "oidc consent deny",
	}); err != nil {
		logger.L.Error().Err(err).Msg("record oidc consent deny")
	}

	return c.JSON(fiber.Map{"redirect": "/login?id=" + req.ID + "&error=access_denied"})
}
