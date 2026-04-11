package handler

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

// AppHandler handles user self-service CRUD for OAuth2 applications.
type AppHandler struct {
	apps  *service.ApplicationService
	audit *store.AuditLogStore
}

type templateApplicationView struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ClientID       string `json:"client_id"`
	RedirectURIs   string `json:"redirect_uris"`
	GrantTypes     string `json:"grant_types"`
	Scopes         string `json:"scopes"`
	RequireConsent bool   `json:"require_consent"`
}

// NewAppHandler creates an AppHandler.
func NewAppHandler(apps *service.ApplicationService, audit *store.AuditLogStore) *AppHandler {
	return &AppHandler{apps: apps, audit: audit}
}

func toTemplateApplicationView(application *service.Application) templateApplicationView {
	if application == nil {
		return templateApplicationView{}
	}
	return templateApplicationView{
		ID:             application.ID,
		Name:           application.Name,
		ClientID:       application.ClientID,
		RedirectURIs:   application.RedirectURIs,
		GrantTypes:     application.GrantTypes,
		Scopes:         application.Scopes,
		RequireConsent: application.RequireConsent,
	}
}

func toTemplateApplicationViews(applications []service.Application) []templateApplicationView {
	views := make([]templateApplicationView, 0, len(applications))
	for i := range applications {
		views = append(views, toTemplateApplicationView(&applications[i]))
	}
	return views
}

// Create handles POST /api/apps.
func (h *AppHandler) Create(c fiber.Ctx) error {
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

	userID, _ := c.Locals("user_id").(string)
	input := service.CreateApplicationInput{
		OwnerUserID:    userID,
		Name:           req.Name,
		RedirectURIs:   req.RedirectURIs,
		GrantTypes:     req.GrantTypes,
		Scopes:         req.Scopes,
		RequireConsent: req.RequireConsent,
	}
	var (
		application *service.Application
		secret      string
		err         error
	)
	application, secret, err = h.apps.Create(c.Context(), input)
	if errors.Is(err, service.ErrApplicationLimitReached) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "maximum number of apps reached (10)"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create app"})
	}

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditAppCreate,
		Resource:   "app",
		ResourceID: application.ID,
		Status:     "success",
		Detail:     "app created",
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"application":   toTemplateApplicationView(application),
		"client":        toTemplateApplicationView(application),
		"client_secret": secret,
	})
}

// List handles GET /api/apps.
func (h *AppHandler) List(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	page, pageSize := parsePagination(c)

	applications, total, err := h.apps.ListByOwner(c.Context(), userID, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list apps"})
	}

	views := toTemplateApplicationViews(applications)
	return c.JSON(fiber.Map{
		"applications": views,
		"clients":      views,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// Get handles GET /api/apps/:id.
func (h *AppHandler) Get(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	application, err := h.apps.GetByIDForOwner(c.Context(), c.Params("id"), userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "app not found"})
	}
	view := toTemplateApplicationView(application)
	return c.JSON(fiber.Map{"application": view, "client": view})
}

// Update handles PUT /api/apps/:id.
func (h *AppHandler) Update(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
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

	input := service.UpdateApplicationInput{
		ID:             c.Params("id"),
		OwnerUserID:    userID,
		Name:           req.Name,
		RedirectURIs:   req.RedirectURIs,
		GrantTypes:     req.GrantTypes,
		Scopes:         req.Scopes,
		RequireConsent: req.RequireConsent,
	}
	var (
		application *service.Application
		err         error
	)
	application, err = h.apps.UpdateByIDForOwner(c.Context(), input)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "app not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update app"})
	}

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditAppUpdate,
		Resource:   "app",
		ResourceID: application.ID,
		Status:     "success",
		Detail:     "app updated",
	})

	view := toTemplateApplicationView(application)
	return c.JSON(fiber.Map{"application": view, "client": view})
}

// Delete handles DELETE /api/apps/:id.
func (h *AppHandler) Delete(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	appID := c.Params("id")
	if err := h.apps.DeleteByIDForOwner(c.Context(), appID, userID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "app not found"})
	}

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditAppDelete,
		Resource:   "app",
		ResourceID: appID,
		Status:     "success",
	})

	return c.JSON(fiber.Map{"message": "app deleted"})
}

// RotateSecret handles POST /api/apps/:id/rotate-secret.
func (h *AppHandler) RotateSecret(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)

	var (
		application *service.Application
		secret      string
		err         error
	)
	application, secret, err = h.apps.RotateSecretByIDForOwner(c.Context(), c.Params("id"), userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "app not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to rotate client secret"})
	}

	metadata := map[string]string{
		"client_id": application.ClientID,
	}
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditAppRotateSecret,
		Resource:   "app",
		ResourceID: application.ID,
		Status:     "success",
		Detail:     "app client secret rotated",
		Metadata:   metadata,
	})

	return c.JSON(fiber.Map{"client_secret": secret})
}

// Stats handles GET /api/apps/stats.
func (h *AppHandler) Stats(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	myLogins, err := h.audit.CountLoginByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get stats"})
	}
	appStats, err := h.audit.AppLoginStats(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get stats"})
	}
	if appStats == nil {
		appStats = []store.AppStat{}
	}
	return c.JSON(fiber.Map{
		"my_login_count": myLogins,
		"app_stats":      appStats,
	})
}
