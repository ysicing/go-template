package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

func TestAdminHandler_GetAuditLogs_FiltersAndReturnsSource(t *testing.T) {
	db := setupTestDB(t)

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	auditStore := store.NewAuditLogStore(db)

	adminUser := createLocalUser(t, db, "audit-admin", "audit-admin@example.com", "Password123!abcd")
	normalUser := createLocalUser(t, db, "audit-user", "audit-user@example.com", "Password123!abcd")

	if err := auditStore.Create(context.Background(), &model.AuditLog{
		UserID:     adminUser.ID,
		Action:     model.AuditSettingUpdate,
		Resource:   "setting",
		ResourceID: "site_title",
		IP:         "10.0.0.1",
		UserAgent:  "admin-browser",
		Detail:     "source=admin message=updated request_id=req-admin",
		Status:     "success",
	}); err != nil {
		t.Fatalf("seed admin audit log: %v", err)
	}
	if err := auditStore.Create(context.Background(), &model.AuditLog{
		UserID:     normalUser.ID,
		Action:     model.AuditAppCreate,
		Resource:   "app",
		ResourceID: "app-1",
		IP:         "10.0.0.2",
		UserAgent:  "api-client",
		Detail:     "source=api channel=api entrypoint=api app_id=app-1",
		Status:     "success",
	}); err != nil {
		t.Fatalf("seed app audit log: %v", err)
	}

	h := NewAdminHandler(AdminDeps{
		Users:   userStore,
		Clients: clientStore,
		Audit:   auditStore,
	})

	app := fiber.New()
	app.Get("/api/admin/audit-logs", h.GetAuditLogs)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?action=app_create&source=api&keyword=app-1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Logs []struct {
			UserID     string `json:"user_id"`
			Username   string `json:"username"`
			Action     string `json:"action"`
			Resource   string `json:"resource"`
			ResourceID string `json:"resource_id"`
			Source     string `json:"source"`
			IP         string `json:"ip"`
			UserAgent  string `json:"user_agent"`
			Detail     string `json:"detail"`
			Status     string `json:"status"`
		} `json:"logs"`
		Total int64 `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body.Total != 1 || len(body.Logs) != 1 {
		t.Fatalf("expected 1 audit log, total=%d len=%d", body.Total, len(body.Logs))
	}
	log := body.Logs[0]
	if log.Action != model.AuditAppCreate {
		t.Fatalf("expected action %q, got %q", model.AuditAppCreate, log.Action)
	}
	if log.Source != model.AuditSourceAPI {
		t.Fatalf("expected source %q, got %q", model.AuditSourceAPI, log.Source)
	}
	if log.UserID != normalUser.ID {
		t.Fatalf("expected user id %q, got %q", normalUser.ID, log.UserID)
	}
	if log.Username != normalUser.Username {
		t.Fatalf("expected username %q, got %q", normalUser.Username, log.Username)
	}
	if log.IP != "10.0.0.2" {
		t.Fatalf("expected ip 10.0.0.2, got %q", log.IP)
	}
}

func TestAdminHandler_GetAuditLogs_FiltersByStatusAndUserID(t *testing.T) {
	db := setupTestDB(t)

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	auditStore := store.NewAuditLogStore(db)

	firstUser := createLocalUser(t, db, "audit-first", "audit-first@example.com", "Password123!abcd")
	secondUser := createLocalUser(t, db, "audit-second", "audit-second@example.com", "Password123!abcd")

	if err := auditStore.Create(context.Background(), &model.AuditLog{
		UserID:     firstUser.ID,
		Action:     model.AuditLoginFailed,
		Resource:   "user",
		ResourceID: firstUser.ID,
		IP:         "10.0.0.3",
		UserAgent:  "failed-login-agent",
		Detail:     "source=api message=invalid password",
		Status:     "failure",
	}); err != nil {
		t.Fatalf("seed failure audit log: %v", err)
	}
	if err := auditStore.Create(context.Background(), &model.AuditLog{
		UserID:     secondUser.ID,
		Action:     model.AuditLogin,
		Resource:   "user",
		ResourceID: secondUser.ID,
		IP:         "10.0.0.4",
		UserAgent:  "success-login-agent",
		Detail:     "source=api message=local",
		Status:     "success",
	}); err != nil {
		t.Fatalf("seed success audit log: %v", err)
	}

	h := NewAdminHandler(AdminDeps{
		Users:   userStore,
		Clients: clientStore,
		Audit:   auditStore,
	})

	app := fiber.New()
	app.Get("/api/admin/audit-logs", h.GetAuditLogs)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/audit-logs?status=failure&user_id="+firstUser.ID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Logs []struct {
			UserID string `json:"user_id"`
			Status string `json:"status"`
			Action string `json:"action"`
		} `json:"logs"`
		Total int64 `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || len(body.Logs) != 1 {
		t.Fatalf("expected 1 audit log, total=%d len=%d", body.Total, len(body.Logs))
	}
	if body.Logs[0].UserID != firstUser.ID {
		t.Fatalf("expected user id %q, got %q", firstUser.ID, body.Logs[0].UserID)
	}
	if body.Logs[0].Status != "failure" {
		t.Fatalf("expected status failure, got %q", body.Logs[0].Status)
	}
	if body.Logs[0].Action != model.AuditLoginFailed {
		t.Fatalf("expected action %q, got %q", model.AuditLoginFailed, body.Logs[0].Action)
	}
}
