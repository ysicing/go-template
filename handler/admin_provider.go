package handler

import (
	"errors"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

type AdminProviderHandler struct {
	providers *store.SocialProviderStore
	audit     *store.AuditLogStore
}

func NewAdminProviderHandler(providers *store.SocialProviderStore, audit *store.AuditLogStore) *AdminProviderHandler {
	return &AdminProviderHandler{providers: providers, audit: audit}
}

func (h *AdminProviderHandler) List(c fiber.Ctx) error {
	providers, err := h.providers.List(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list providers"})
	}
	// Mask sensitive fields before returning
	for i := range providers {
		if providers[i].ClientSecret != "" {
			providers[i].ClientSecret = "***REDACTED***"
		}
	}
	return c.JSON(fiber.Map{"providers": providers})
}

func (h *AdminProviderHandler) Get(c fiber.Ctx) error {
	provider, err := h.providers.GetByID(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, store.ErrSocialProviderSecretUnavailable) {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "provider secret unavailable"})
		}
		if errors.Is(err, store.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "provider not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load provider"})
	}
	// Mask sensitive fields before returning
	if provider.ClientSecret != "" {
		provider.ClientSecret = "***REDACTED***"
	}
	return c.JSON(fiber.Map{"provider": provider})
}

func (h *AdminProviderHandler) Create(c fiber.Ctx) error {
	var req struct {
		Name         string `json:"name"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RedirectURL  string `json:"redirect_url"`
		Enabled      bool   `json:"enabled"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" || req.ClientID == "" || req.ClientSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name, client_id and client_secret are required"})
	}
	provider := &model.SocialProvider{
		Name:         req.Name,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURL:  req.RedirectURL,
		Enabled:      req.Enabled,
	}
	if err := h.providers.Upsert(c.Context(), provider); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create provider"})
	}
	adminID, _ := c.Locals("user_id").(string)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: adminID, Action: model.AuditProviderCreate, Resource: "social_provider", ResourceID: provider.ID,
		IP: GetRealIP(c), UserAgent: c.Get("User-Agent"), Status: "success", Detail: provider.Name,
	})
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"provider": provider})
}

func (h *AdminProviderHandler) Update(c fiber.Ctx) error {
	provider, err := h.providers.GetByID(c.Context(), c.Params("id"))
	if err != nil {
		if errors.Is(err, store.ErrSocialProviderSecretUnavailable) {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "provider secret unavailable"})
		}
		if errors.Is(err, store.ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "provider not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load provider"})
	}
	var req struct {
		ClientID     *string `json:"client_id"`
		ClientSecret *string `json:"client_secret"`
		RedirectURL  *string `json:"redirect_url"`
		Enabled      *bool   `json:"enabled"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.ClientID != nil {
		provider.ClientID = *req.ClientID
	}
	if req.ClientSecret != nil && *req.ClientSecret != "" {
		provider.ClientSecret = *req.ClientSecret
	}
	if req.RedirectURL != nil {
		provider.RedirectURL = *req.RedirectURL
	}
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}
	if err := h.providers.Upsert(c.Context(), provider); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update provider"})
	}
	adminID, _ := c.Locals("user_id").(string)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: adminID, Action: model.AuditProviderUpdate, Resource: "social_provider", ResourceID: provider.ID,
		IP: GetRealIP(c), UserAgent: c.Get("User-Agent"), Status: "success", Detail: provider.Name,
	})
	return c.JSON(fiber.Map{"provider": provider})
}

func (h *AdminProviderHandler) Delete(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.providers.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete provider"})
	}
	adminID, _ := c.Locals("user_id").(string)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: adminID, Action: model.AuditProviderDelete, Resource: "social_provider", ResourceID: id,
		IP: GetRealIP(c), UserAgent: c.Get("User-Agent"), Status: "success",
	})
	return c.JSON(fiber.Map{"message": "provider deleted"})
}
