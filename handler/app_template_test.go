package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/store"
)

func newTemplateAppHandlerTestApp(t *testing.T, h *AppHandler, userID string) *fiber.App {
	t.Helper()

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("principal_type", "user")
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Post("/api/apps", h.Create)
	app.Get("/api/apps", h.List)
	return app
}

func TestAppHandlerTemplateCreateReturnsApplicationEnvelope(t *testing.T) {
	db := setupTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	h := NewAppHandler(service.NewApplicationService(clients), audit)
	app := newTemplateAppHandlerTestApp(t, h, "user-1")

	payload, err := json.Marshal(map[string]any{
		"name":          "Portal",
		"redirect_uris": "https://portal.example.com/callback",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Contains(t, body, "application")
	require.Contains(t, body, "client")
	require.Contains(t, body, "client_secret")
}

func TestAppHandlerTemplateListReturnsApplicationCollection(t *testing.T) {
	db := setupTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	h := NewAppHandler(service.NewApplicationService(clients), audit)
	app := newTemplateAppHandlerTestApp(t, h, "user-1")

	createReq := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewBufferString(`{"name":"Portal","redirect_uris":"https://portal.example.com/callback"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	listReq := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&body))
	require.Contains(t, body, "applications")
	require.Contains(t, body, "clients")
}

func TestAppHandlerTemplateCreatePersistsRequireConsent(t *testing.T) {
	db := setupTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	h := NewAppHandler(service.NewApplicationService(clients), audit)
	app := newTemplateAppHandlerTestApp(t, h, "user-1")

	payload, err := json.Marshal(map[string]any{
		"name":            "Portal",
		"redirect_uris":   "https://portal.example.com/callback",
		"require_consent": true,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body struct {
		Application struct {
			RequireConsent bool `json:"require_consent"`
		} `json:"application"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.True(t, body.Application.RequireConsent)
}

func TestAppHandlerTemplateCreateIgnoresOrganizationScopeFields(t *testing.T) {
	db := setupTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	h := NewAppHandler(service.NewApplicationService(clients), audit)
	app := newTemplateAppHandlerTestApp(t, h, "user-1")

	payload, err := json.Marshal(map[string]any{
		"name":            "Portal",
		"redirect_uris":   "https://portal.example.com/callback",
		"organization_id": "org-should-be-ignored",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/apps", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body struct {
		Application map[string]any `json:"application"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotContains(t, body.Application, "organization_id")
	require.NotContains(t, body.Application, "workspace")
}

func TestAppHandlerTemplateListOmitsOrganizationScopeFields(t *testing.T) {
	db := setupTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	h := NewAppHandler(service.NewApplicationService(clients), audit)
	app := newTemplateAppHandlerTestApp(t, h, "user-1")

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/apps",
		bytes.NewBufferString(`{"name":"Portal","redirect_uris":"https://portal.example.com/callback","organization_id":"org-ignored"}`),
	)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	listReq := httptest.NewRequest(http.MethodGet, "/api/apps", nil)
	listResp, err := app.Test(listReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var body struct {
		Applications []map[string]any `json:"applications"`
	}
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&body))
	require.Len(t, body.Applications, 1)
	require.NotContains(t, body.Applications[0], "organization_id")
	require.NotContains(t, body.Applications[0], "workspace")
}
