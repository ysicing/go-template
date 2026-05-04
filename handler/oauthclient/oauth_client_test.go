package oauthclienthandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func newOAuthClientTestApp(t *testing.T, h *OAuthClientHandler, userID string) *fiber.App {
	t.Helper()

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Post("/api/admin/clients", h.Create)
	app.Put("/api/admin/clients/:id", h.Update)
	app.Delete("/api/admin/clients/:id", h.Delete)
	return app
}

func TestOAuthClientHandler_WriteOperationsCreateAuditLogs(t *testing.T) {
	db := setupTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	admin := createLocalUser(t, db, "oauth-admin", "oauth-admin@example.com", "Password123!abcd")

	h := NewOAuthClientHandler(clients, audit)
	app := newOAuthClientTestApp(t, h, admin.ID)

	createPayload, _ := json.Marshal(map[string]any{
		"name":        "Audit Client",
		"grant_types": "client_credentials",
		"scopes":      "read,write",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/clients", bytes.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatal(err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	var createdBody struct {
		Client model.OAuthClient `json:"client"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createdBody); err != nil {
		t.Fatal(err)
	}

	updatePayload, _ := json.Marshal(map[string]any{
		"name":   "Audit Client Updated",
		"scopes": "read",
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/admin/clients/"+createdBody.Client.ID, bytes.NewReader(updatePayload))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := app.Test(updateReq)
	if err != nil {
		t.Fatal(err)
	}
	if updateResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", updateResp.StatusCode)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/admin/clients/"+createdBody.Client.ID, nil)
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatal(err)
	}
	if deleteResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", deleteResp.StatusCode)
	}

	assertAuditActionExists(t, db, admin.ID, model.AuditAppCreate, "oauth_client")
	assertAuditActionExists(t, db, admin.ID, model.AuditAppUpdate, "oauth_client")
	assertAuditActionExists(t, db, admin.ID, model.AuditAppDelete, "oauth_client")
}

func TestOAuthClientHandlerCreateDefaultsToMachineGrant(t *testing.T) {
	db := setupTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	admin := createLocalUser(t, db, "oauth-default-admin", "oauth-default-admin@example.com", "Password123!abcd")

	h := NewOAuthClientHandler(clients, audit)
	app := newOAuthClientTestApp(t, h, admin.ID)

	createPayload, _ := json.Marshal(map[string]any{
		"name": "Machine Client",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/clients", bytes.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatal(err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}

	var body struct {
		Client model.OAuthClient `json:"client"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Client.GrantTypes != "client_credentials" {
		t.Fatalf("expected default grant_types client_credentials, got %q", body.Client.GrantTypes)
	}
	if body.Client.Scopes != "read" {
		t.Fatalf("expected default scopes read, got %q", body.Client.Scopes)
	}
}

func TestOAuthClientHandlerRejectsUnsupportedGrantType(t *testing.T) {
	db := setupTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	admin := createLocalUser(t, db, "oauth-invalid-admin", "oauth-invalid-admin@example.com", "Password123!abcd")

	h := NewOAuthClientHandler(clients, audit)
	app := newOAuthClientTestApp(t, h, admin.ID)

	createPayload, _ := json.Marshal(map[string]any{
		"name":        "Invalid Client",
		"grant_types": "authorization_code",
		"scopes":      "read",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/clients", bytes.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatal(err)
	}
	if createResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", createResp.StatusCode)
	}
}

func assertAuditActionExists(t *testing.T, db *gorm.DB, userID, action, resource string) {
	t.Helper()

	var auditLog model.AuditLog
	if err := db.WithContext(context.Background()).
		Where("user_id = ? AND action = ? AND resource = ?", userID, action, resource).
		First(&auditLog).Error; err != nil {
		t.Fatalf("expected audit log for action %s: %v", action, err)
	}
}
