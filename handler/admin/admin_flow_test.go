package adminhandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

func TestAdminUpdateUser_UsesJSONPermissions(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	auditStore := store.NewAuditLogStore(db)

	admin := createLocalUser(t, db, "admin-update", "admin-update@example.com", "Password123!abcd")
	admin.IsAdmin = true
	admin.SetPermissions(model.AllAdminPermissions())
	if err := db.Save(admin).Error; err != nil {
		t.Fatalf("save admin: %v", err)
	}

	target := createLocalUser(t, db, "target-update", "target-update@example.com", "Password123!abcd")

	cache := store.NewMemoryCache()
	defer cache.Close()

	h := NewAdminHandler(AdminDeps{
		Users:   userStore,
		Clients: clientStore,
		Audit:   auditStore,
		Cache:   cache,
	})
	app := fiber.New()
	app.Put("/api/admin/users/:id", func(c fiber.Ctx) error {
		c.Locals("user_id", admin.ID)
		return h.UpdateUser(c)
	})

	payload, _ := json.Marshal(map[string]any{
		"permissions": []string{model.PermissionAdminUsersRead, model.PermissionAdminStatsRead},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+target.ID, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	updated, err := userStore.GetByID(ctx, target.ID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updated.IsAdmin {
		t.Fatal("expected target user to not be admin when custom permissions are set")
	}
	if !updated.HasPermission(model.PermissionAdminUsersRead) {
		t.Fatal("expected target user to have admin.users.read permission")
	}
	if !updated.HasPermission(model.PermissionAdminStatsRead) {
		t.Fatal("expected target user to have admin.stats.read permission")
	}
}

func TestAdminUpdateUser_InvalidatesPermissionCache(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	auditStore := store.NewAuditLogStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	admin := createLocalUser(t, db, "admin-cache", "admin-cache@example.com", "Password123!abcd")
	admin.IsAdmin = true
	admin.SetPermissions(model.AllAdminPermissions())
	if err := db.Save(admin).Error; err != nil {
		t.Fatalf("save admin: %v", err)
	}

	target := createLocalUser(t, db, "target-cache", "target-cache@example.com", "Password123!abcd")

	cacheKey := "perm_check:" + target.ID + ":" + model.PermissionAdminUsersRead
	if err := cache.Set(ctx, cacheKey, "0", 30*time.Second); err != nil {
		t.Fatalf("set permission cache: %v", err)
	}

	h := NewAdminHandler(AdminDeps{
		Users:   userStore,
		Clients: clientStore,
		Audit:   auditStore,
		Cache:   cache,
	})
	app := fiber.New()
	app.Put("/api/admin/users/:id", func(c fiber.Ctx) error {
		c.Locals("user_id", admin.ID)
		return h.UpdateUser(c)
	})

	payload, _ := json.Marshal(map[string]any{
		"permissions": []string{model.PermissionAdminUsersRead},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+target.ID, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	if _, err := cache.Get(ctx, cacheKey); err != store.ErrCacheMiss {
		t.Fatalf("expected permission cache to be invalidated, got err=%v", err)
	}
}

func TestAdminUpdateUser_BumpsTokenVersionAndInvalidatesTokenVersionCache(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	auditStore := store.NewAuditLogStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	admin := createLocalUser(t, db, "admin-ver-bump", "admin-ver-bump@example.com", "Password123!abcd")
	admin.IsAdmin = true
	admin.SetPermissions(model.AllAdminPermissions())
	if err := db.Save(admin).Error; err != nil {
		t.Fatalf("save admin: %v", err)
	}

	target := createLocalUser(t, db, "target-ver-bump", "target-ver-bump@example.com", "Password123!abcd")
	target.TokenVersion = 1
	target.SetPermissions([]string{model.PermissionAdminUsersRead})
	if err := db.Save(target).Error; err != nil {
		t.Fatalf("save target: %v", err)
	}

	if err := cache.Set(ctx, "token_ver:"+target.ID, "1", 30*time.Second); err != nil {
		t.Fatalf("seed token version cache: %v", err)
	}

	h := NewAdminHandler(AdminDeps{
		Users:   userStore,
		Clients: clientStore,
		Audit:   auditStore,
		Cache:   cache,
	})
	app := fiber.New()
	app.Put("/api/admin/users/:id", func(c fiber.Ctx) error {
		c.Locals("user_id", admin.ID)
		return h.UpdateUser(c)
	})

	payload, _ := json.Marshal(map[string]any{
		"permissions": []string{model.PermissionAdminStatsRead},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/admin/users/"+target.ID, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	updated, err := userStore.GetByID(ctx, target.ID)
	if err != nil {
		t.Fatalf("reload target: %v", err)
	}
	if updated.TokenVersion != 2 {
		t.Fatalf("expected token_version=2, got %d", updated.TokenVersion)
	}

	if _, err := cache.Get(ctx, "token_ver:"+target.ID); err != store.ErrCacheMiss {
		t.Fatalf("expected token version cache invalidated, got err=%v", err)
	}
}

func TestAdminDeleteUser_RevokesRefreshTokens(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	auditStore := store.NewAuditLogStore(db)
	refreshStore := store.NewAPIRefreshTokenStore(db)

	admin := createLocalUser(t, db, "admin-del", "admin-del@example.com", "Password123!abcd")
	admin.IsAdmin = true
	db.Save(admin)

	target := createLocalUser(t, db, "target-del", "target-del@example.com", "Password123!abcd")

	cache := store.NewMemoryCache()
	defer cache.Close()

	tokenHash := store.HashToken("target-refresh-token")
	if err := refreshStore.Create(ctx, &model.APIRefreshToken{
		UserID: target.ID, TokenHash: tokenHash, ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	h := NewAdminHandler(AdminDeps{
		Users:         userStore,
		Clients:       clientStore,
		Audit:         auditStore,
		RefreshTokens: refreshStore,
		Cache:         cache,
	})
	app := fiber.New()
	app.Delete("/api/admin/users/:id", func(c fiber.Ctx) error {
		c.Locals("user_id", admin.ID)
		return h.DeleteUser(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+target.ID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	if _, err := refreshStore.GetByTokenHash(ctx, tokenHash); err == nil {
		t.Fatal("expected refresh token to be revoked after user deletion")
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
