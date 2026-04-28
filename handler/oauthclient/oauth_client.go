package oauthclienthandler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// allowedGrantTypes is the set of permitted OAuth2 grant types.
var allowedGrantTypes = map[string]bool{
	"authorization_code": true,
	"refresh_token":      true,
	"client_credentials": true,
}

// allowedScopes is the set of permitted OAuth2 scopes.
var allowedScopes = map[string]bool{
	"openid":  true,
	"profile": true,
	"email":   true,
	"admin":   true,
}

// validateGrantTypes checks that all comma-separated grant types are allowed.
func validateGrantTypes(raw string) error {
	if raw == "" {
		return nil
	}
	for _, g := range strings.Split(raw, ",") {
		g = strings.TrimSpace(g)
		if g != "" && !allowedGrantTypes[g] {
			return fmt.Errorf("unsupported grant type: %s", g)
		}
	}
	return nil
}

// validateScopes checks that all comma-separated scopes are allowed.
func validateScopes(raw string) error {
	if raw == "" {
		return nil
	}
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" && !allowedScopes[s] {
			return fmt.Errorf("unsupported scope: %s", s)
		}
	}
	return nil
}

// OAuthClientHandler handles admin CRUD for OAuth2 client applications.
type OAuthClientHandler struct {
	clients *store.OAuthClientStore
	audit   *store.AuditLogStore
}

// NewOAuthClientHandler creates an OAuthClientHandler.
func NewOAuthClientHandler(clients *store.OAuthClientStore, audit *store.AuditLogStore) *OAuthClientHandler {
	return &OAuthClientHandler{clients: clients, audit: audit}
}

// generateSecret returns a cryptographically random 32-byte hex string.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create handles POST /api/admin/clients.
func (h *OAuthClientHandler) Create(c fiber.Ctx) error {
	var req struct {
		Name           string `json:"name"`
		RedirectURIs   string `json:"redirect_uris"`
		GrantTypes     string `json:"grant_types"`
		Scopes         string `json:"scopes"`
		RequireConsent bool   `json:"require_consent"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	req.Name = strings.TrimSpace(req.Name)
	req.RedirectURIs = strings.TrimSpace(req.RedirectURIs)

	if req.Name == "" || req.RedirectURIs == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name and redirect_uris are required"})
	}

	if err := validateGrantTypes(req.GrantTypes); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := validateScopes(req.Scopes); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	secret, err := generateSecret()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate client secret"})
	}

	userID, _ := c.Locals("user_id").(string)

	client := &model.OAuthClient{
		Name:           req.Name,
		ClientID:       uuid.New().String(),
		RedirectURIs:   req.RedirectURIs,
		GrantTypes:     req.GrantTypes,
		Scopes:         req.Scopes,
		RequireConsent: req.RequireConsent,
		UserID:         userID,
	}
	if err := client.SetSecret(secret); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash client secret"})
	}
	if client.GrantTypes == "" {
		client.GrantTypes = "authorization_code"
	}
	if client.Scopes == "" {
		client.Scopes = "openid profile email"
	}

	if err := h.clients.Create(c.Context(), client); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create client"})
	}

	adminID, _ := c.Locals("user_id").(string)
	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
		UserID:     adminID,
		Action:     model.AuditAppCreate,
		Resource:   "oauth_client",
		ResourceID: client.ID,
		Status:     "success",
		Detail:     "oauth client created",
		Metadata: map[string]string{
			"client_id": client.ClientID,
		},
	})

	// Return secret in create response since json:"-" hides it in subsequent GETs.
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"client":        client,
		"client_secret": secret,
	})
}

// List handles GET /api/admin/clients.
func (h *OAuthClientHandler) List(c fiber.Ctx) error {
	page, pageSize := handlercommon.ParsePagination(c)

	clients, total, err := h.clients.List(c.Context(), page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list clients"})
	}

	return c.JSON(fiber.Map{
		"clients":   clients,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Get handles GET /api/admin/clients/:id.
func (h *OAuthClientHandler) Get(c fiber.Ctx) error {
	id := c.Params("id")
	client, err := h.clients.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "client not found"})
	}
	return c.JSON(fiber.Map{"client": client})
}

// Update handles PUT /api/admin/clients/:id.
func (h *OAuthClientHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	client, err := h.clients.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "client not found"})
	}

	var req struct {
		Name           *string `json:"name"`
		RedirectURIs   *string `json:"redirect_uris"`
		GrantTypes     *string `json:"grant_types"`
		Scopes         *string `json:"scopes"`
		RequireConsent *bool   `json:"require_consent"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.GrantTypes != nil {
		if err := validateGrantTypes(*req.GrantTypes); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}
	if req.Scopes != nil {
		if err := validateScopes(*req.Scopes); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if req.Name != nil {
		client.Name = strings.TrimSpace(*req.Name)
	}
	if req.RedirectURIs != nil {
		client.RedirectURIs = strings.TrimSpace(*req.RedirectURIs)
	}
	if req.GrantTypes != nil {
		client.GrantTypes = *req.GrantTypes
	}
	if req.Scopes != nil {
		client.Scopes = *req.Scopes
	}
	if req.RequireConsent != nil {
		client.RequireConsent = *req.RequireConsent
	}

	if err := h.clients.Update(c.Context(), client); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update client"})
	}

	adminID, _ := c.Locals("user_id").(string)
	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
		UserID:     adminID,
		Action:     model.AuditAppUpdate,
		Resource:   "oauth_client",
		ResourceID: client.ID,
		Status:     "success",
		Detail:     "oauth client updated",
		Metadata: map[string]string{
			"client_id": client.ClientID,
		},
	})

	return c.JSON(fiber.Map{"client": client})
}

// Delete handles DELETE /api/admin/clients/:id.
func (h *OAuthClientHandler) Delete(c fiber.Ctx) error {
	id := c.Params("id")
	client, err := h.clients.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "client not found"})
	}

	if err := h.clients.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete client"})
	}

	adminID, _ := c.Locals("user_id").(string)
	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
		UserID:     adminID,
		Action:     model.AuditAppDelete,
		Resource:   "oauth_client",
		ResourceID: id,
		Status:     "success",
		Detail:     "oauth client deleted",
		Metadata: map[string]string{
			"client_id": client.ClientID,
		},
	})

	return c.JSON(fiber.Map{"message": "client deleted"})
}
