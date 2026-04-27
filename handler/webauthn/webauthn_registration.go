package webauthnhandler

import (
	"encoding/json"
	"time"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
	webauthnstore "github.com/ysicing/go-template/store/webauthn"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

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
	h.saveSessionData(c.Context(), "webauthn_reg:"+userID, session)
	return c.JSON(options)
}

// RegisterFinish handles POST /api/mfa/webauthn/register/finish.
func (h *WebAuthnHandler) RegisterFinish(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	waUser, err := h.loadWebAuthnUser(c, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	session, err := h.loadRegistrationSession(c, userID)
	if err != nil {
		return err
	}
	parsedResponse, err := loadCredentialCreationResponse(c.Body())
	if err != nil {
		logger.L.Error().Err(err).Msg("webauthn parse credential creation response")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid credential response"})
	}
	dbCred, err := h.buildWebAuthnCredential(c, userID, waUser, session, parsedResponse)
	if err != nil {
		return err
	}
	if err := h.creds.Create(c.Context(), dbCred); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save credential"})
	}
	h.writeCredentialAddedAudit(c, userID, dbCred.ID)
	return c.JSON(fiber.Map{"message": "credential registered", "id": dbCred.ID})
}

// DeleteCredential handles DELETE /api/mfa/webauthn/credentials/:id.
func (h *WebAuthnHandler) DeleteCredential(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	credID := c.Params("id")
	if err := h.creds.Delete(c.Context(), credID, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete credential"})
	}
	waDelIP, waDelUA := handlercommon.GetRealIPAndUA(c)
	_ = handlercommon.WriteAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditWebAuthnRemove, Resource: "webauthn", ResourceID: credID,
		IP: waDelIP, UserAgent: waDelUA, Status: "success",
	})
	return c.JSON(fiber.Map{"message": "credential deleted"})
}

func (h *WebAuthnHandler) loadRegistrationSession(c fiber.Ctx, userID string) (session webauthn.SessionData, err error) {
	sessionJSON, err := h.cache.Get(c.Context(), "webauthn_reg:"+userID)
	if err != nil || sessionJSON == "" {
		return session, fiber.NewError(fiber.StatusBadRequest, "no pending registration")
	}
	session, err = loadSessionData(sessionJSON)
	if err != nil {
		return session, fiber.NewError(fiber.StatusBadRequest, "invalid session")
	}
	return session, nil
}

func (h *WebAuthnHandler) buildWebAuthnCredential(
	c fiber.Ctx,
	userID string,
	waUser *webauthnstore.WebAuthnUser,
	session webauthn.SessionData,
	parsedResponse *protocol.ParsedCredentialCreationData,
) (*model.WebAuthnCredential, error) {
	wa, err := h.getWebAuthn()
	if err != nil {
		return nil, fiber.NewError(fiber.StatusServiceUnavailable, "webauthn not configured")
	}
	credential, err := wa.CreateCredential(waUser, session, parsedResponse)
	if err != nil {
		logger.L.Error().Err(err).Msg("webauthn create credential")
		return nil, fiber.NewError(fiber.StatusBadRequest, "failed to create credential")
	}

	_ = h.cache.Del(c.Context(), "webauthn_reg:"+userID)
	transportsJSON, _ := json.Marshal(credential.Transport)
	return &model.WebAuthnCredential{
		UserID:         userID,
		Name:           normalizeWebAuthnCredentialName(c.Query("name")),
		CredentialID:   credential.ID,
		PublicKey:      credential.PublicKey,
		AAGUID:         credential.Authenticator.AAGUID,
		SignCount:      credential.Authenticator.SignCount,
		BackupEligible: credential.Flags.BackupEligible,
		BackupState:    credential.Flags.BackupState,
		Transport:      string(transportsJSON),
	}, nil
}

func (h *WebAuthnHandler) writeCredentialAddedAudit(c fiber.Ctx, userID, credentialID string) {
	waAddIP, waAddUA := handlercommon.GetRealIPAndUA(c)
	_ = handlercommon.WriteAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditWebAuthnAdd, Resource: "webauthn", ResourceID: credentialID,
		IP: waAddIP, UserAgent: waAddUA, Status: "success",
	})
}
