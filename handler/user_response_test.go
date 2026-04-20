package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

func TestUserHandler_GetMe_IncludesPermissionList(t *testing.T) {
	db := setupTestDB(t)
	userStore := store.NewUserStore(db)
	user := createLocalUser(t, db, "perm-user", "perm-user@example.com", "Password123!abcd")
	user.SetPermissions([]string{model.PermissionAdminStatsRead})
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save permissions: %v", err)
	}

	h := NewUserHandler(UserDeps{Users: userStore})
	app := fiber.New()
	app.Get("/api/users/me", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return h.GetMe(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		User struct {
			ID          string   `json:"id"`
			Permissions []string `json:"permissions"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.User.ID != user.ID {
		t.Fatalf("expected user id %q, got %q", user.ID, body.User.ID)
	}
	if len(body.User.Permissions) != 1 || body.User.Permissions[0] != model.PermissionAdminStatsRead {
		t.Fatalf("expected permissions [%s], got %#v", model.PermissionAdminStatsRead, body.User.Permissions)
	}
}

func TestAdminUpdateUser_ClearsPermissionsWithEmptyArray(t *testing.T) {
	db := setupTestDB(t)
	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	auditStore := store.NewAuditLogStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	admin := createLocalUser(t, db, "admin-clear-perms", "admin-clear-perms@example.com", "Password123!abcd")
	admin.IsAdmin = true
	admin.SetPermissions(model.AllAdminPermissions())
	if err := db.Save(admin).Error; err != nil {
		t.Fatalf("save admin: %v", err)
	}

	target := createLocalUser(t, db, "target-clear-perms", "target-clear-perms@example.com", "Password123!abcd")
	target.SetPermissions([]string{model.PermissionAdminUsersRead})
	if err := db.Save(target).Error; err != nil {
		t.Fatalf("save target permissions: %v", err)
	}

	h := NewAdminHandler(AdminDeps{
		Users:   userStore,
		Clients: clientStore,
		Audit:   auditStore,
		Cache:   cache,
		DB:      db,
	})
	app := fiber.New()
	app.Put("/api/admin/users/:id", func(c fiber.Ctx) error {
		c.Locals("user_id", admin.ID)
		return h.UpdateUser(c)
	})

	payload, _ := json.Marshal(map[string]any{"permissions": []string{}})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+target.ID, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	updated, err := userStore.GetByID(t.Context(), target.ID)
	if err != nil {
		t.Fatalf("reload target: %v", err)
	}
	if perms := updated.PermissionList(); len(perms) != 0 {
		t.Fatalf("expected permissions to be cleared, got %#v", perms)
	}
}
