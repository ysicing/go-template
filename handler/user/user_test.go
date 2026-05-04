package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auditcore "github.com/ysicing/go-template/internal/audit"
	httpmiddleware "github.com/ysicing/go-template/internal/http/middleware"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func setupUserHandler(t *testing.T) (*UserHandler, *store.UserStore, *store.APIRefreshTokenStore, *store.AuditLogStore, *model.User, *gorm.DB) {
	t.Helper()

	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	users := store.NewUserStore(db)
	passwordHistory := store.NewPasswordHistoryStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)
	audit := store.NewAuditLogStore(db)
	settings := store.NewSettingStore(db, cache)
	user := createLocalUser(t, db, "profile-user", "profile-user@example.com", "Password123!abcd")

	h := NewUserHandler(UserDeps{
		Users:           users,
		PasswordHistory: passwordHistory,
		RefreshTokens:   refreshTokens,
		Audit:           audit,
		Cache:           cache,
		Settings:        settings,
	})
	return h, users, refreshTokens, audit, user, db
}

func newUserTestApp(t *testing.T, h *UserHandler, userID string) *fiber.App {
	t.Helper()

	app := fiber.New()
	app.Use(httpmiddleware.RequestIDMiddleware())
	app.Use(auditcore.AuditContextMiddleware())
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Put("/api/users/me", h.UpdateMe)
	app.Put("/api/users/me/password", h.ChangePassword)
	app.Delete("/api/sessions/:id", h.RevokeSession)
	app.Delete("/api/sessions", h.RevokeAllSessions)
	app.Post("/api/users/me/set-password", h.SetPassword)
	return app
}

func TestUserHandler_UpdateMe_WritesAuditLog(t *testing.T) {
	h, _, _, _, user, db := setupUserHandler(t)
	app := newUserTestApp(t, h, user.ID)

	payload, _ := json.Marshal(map[string]string{"username": "profile-user-updated"})
	req := httptest.NewRequest(http.MethodPut, "/api/users/me", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	assertUserAuditLogByAction(t, db, user.ID, model.AuditUserUpdate, "user")
}

func TestUserHandler_UpdateMe_RejectsInvalidEmailWithoutDomain(t *testing.T) {
	h, _, _, _, user, _ := setupUserHandler(t)
	app := newUserTestApp(t, h, user.ID)

	payload, _ := json.Marshal(map[string]string{"email": "invalid@local"})
	req := httptest.NewRequest(http.MethodPut, "/api/users/me", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUserHandler_RevokeSession_WritesAuditLog(t *testing.T) {
	h, _, refreshTokens, _, user, db := setupUserHandler(t)
	app := newUserTestApp(t, h, user.ID)

	token := &model.APIRefreshToken{
		UserID:     user.ID,
		TokenHash:  store.HashToken("session-token"),
		Family:     "family-1",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IP:         "127.0.0.1",
		UserAgent:  "test-agent",
		LastUsedAt: time.Now(),
	}
	if err := refreshTokens.Create(context.Background(), token); err != nil {
		t.Fatalf("seed refresh token: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+token.ID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	assertUserAuditLogByAction(t, db, user.ID, model.AuditSessionRevoke, "session")
}

func TestUserHandler_SetPassword_WritesAuditLog(t *testing.T) {
	h, users, _, _, user, db := setupUserHandler(t)
	app := newUserTestApp(t, h, user.ID)

	user.PasswordHash = ""
	if err := users.Update(context.Background(), user); err != nil {
		t.Fatalf("clear password: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{"password": "NewPassword123!@"})
	req := httptest.NewRequest(http.MethodPost, "/api/users/me/set-password", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	assertUserAuditLogByAction(t, db, user.ID, model.AuditPasswordSet, "user")
}

func TestUserHandler_SetPassword_AllowsWeakPasswordWhenPolicyDisabled(t *testing.T) {
	h, users, _, _, user, db := setupUserHandler(t)

	settings := store.NewSettingStore(db, store.NewMemoryCache())
	_ = settings.Set(context.Background(), store.SettingPasswordPolicyEnabled, "false")
	h.settings = settings

	app := newUserTestApp(t, h, user.ID)

	user.PasswordHash = ""
	if err := users.Update(context.Background(), user); err != nil {
		t.Fatalf("clear password: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{"password": "weak"})
	req := httptest.NewRequest(http.MethodPost, "/api/users/me/set-password", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUserHandler_ChangePassword_BumpsTokenVersionAndClearsSessions(t *testing.T) {
	h, users, refreshTokens, _, user, _ := setupUserHandler(t)
	app := newUserTestApp(t, h, user.ID)
	ctx := context.Background()

	user.TokenVersion = 1
	if err := users.Update(ctx, user); err != nil {
		t.Fatalf("seed token version: %v", err)
	}
	if err := h.cache.Set(ctx, "token_ver:"+user.ID, "1", 30*time.Second); err != nil {
		t.Fatalf("seed token version cache: %v", err)
	}

	token := &model.APIRefreshToken{
		UserID:     user.ID,
		TokenHash:  store.HashToken("change-password-token"),
		Family:     "family-change-password",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IP:         "127.0.0.1",
		UserAgent:  "test-agent",
		LastUsedAt: time.Now(),
	}
	if err := refreshTokens.Create(ctx, token); err != nil {
		t.Fatalf("seed refresh token: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"current_password": "Password123!abcd",
		"new_password":     "NewPassword123!@#",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/users/me/password", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updated, err := users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.TokenVersion != 2 {
		t.Fatalf("expected token version 2, got %d", updated.TokenVersion)
	}
	if _, err := h.cache.Get(ctx, "token_ver:"+user.ID); err != store.ErrCacheMiss {
		t.Fatalf("expected token version cache cleared, got err=%v", err)
	}
	if _, err := refreshTokens.GetByTokenHash(ctx, token.TokenHash); err == nil {
		t.Fatalf("expected refresh token revoked after password change")
	}
}

func TestUserHandler_RevokeAllSessions_BumpsTokenVersion(t *testing.T) {
	h, users, refreshTokens, _, user, _ := setupUserHandler(t)
	app := newUserTestApp(t, h, user.ID)
	ctx := context.Background()

	user.TokenVersion = 1
	if err := users.Update(ctx, user); err != nil {
		t.Fatalf("seed token version: %v", err)
	}
	if err := h.cache.Set(ctx, "token_ver:"+user.ID, "1", 30*time.Second); err != nil {
		t.Fatalf("seed token version cache: %v", err)
	}

	first := &model.APIRefreshToken{
		UserID:     user.ID,
		TokenHash:  store.HashToken("revoke-all-1"),
		Family:     "family-revoke-all",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IP:         "127.0.0.1",
		UserAgent:  "test-agent",
		LastUsedAt: time.Now(),
	}
	second := &model.APIRefreshToken{
		UserID:     user.ID,
		TokenHash:  store.HashToken("revoke-all-2"),
		Family:     "family-revoke-all",
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		IP:         "127.0.0.1",
		UserAgent:  "test-agent",
		LastUsedAt: time.Now(),
	}
	if err := refreshTokens.Create(ctx, first); err != nil {
		t.Fatalf("seed first refresh token: %v", err)
	}
	if err := refreshTokens.Create(ctx, second); err != nil {
		t.Fatalf("seed second refresh token: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updated, err := users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.TokenVersion != 2 {
		t.Fatalf("expected token version 2, got %d", updated.TokenVersion)
	}
	if _, err := h.cache.Get(ctx, "token_ver:"+user.ID); err != store.ErrCacheMiss {
		t.Fatalf("expected token version cache cleared, got err=%v", err)
	}
	tokens, err := refreshTokens.ListByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("list refresh tokens: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected all refresh tokens revoked, got %d", len(tokens))
	}
}

func TestUserHandler_RevokeAllSessions_ReturnsErrorWhenUserMissing(t *testing.T) {
	h, _, _, _, _, _ := setupUserHandler(t)
	app := newUserTestApp(t, h, "missing-user-id")

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func assertUserAuditLogByAction(t *testing.T, db *gorm.DB, userID, action, resource string) {
	t.Helper()

	var auditLog model.AuditLog
	if err := db.WithContext(context.Background()).
		Where("user_id = ? AND action = ? AND resource = ?", userID, action, resource).
		Order("created_at DESC").
		First(&auditLog).Error; err != nil {
		t.Fatalf("expected audit log for action %s: %v", action, err)
	}
}
