package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

func setupUserHandler(t *testing.T) (*UserHandler, *store.UserStore, *store.APIRefreshTokenStore, *store.AuditLogStore, *store.OAuthConsentGrantStore, *store.OAuthClientStore, *model.User, *gorm.DB) {
	t.Helper()

	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })
	users := store.NewUserStore(db)
	passwordHistory := store.NewPasswordHistoryStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)
	audit := store.NewAuditLogStore(db)
	consentGrants := store.NewOAuthConsentGrantStore(db)
	clients := store.NewOAuthClientStore(db)
	settings := store.NewSettingStore(db, cache)
	user := createLocalUser(t, db, "profile-user", "profile-user@example.com", "Password123!abcd")

	h := NewUserHandler(UserDeps{
		Users:           users,
		PasswordHistory: passwordHistory,
		RefreshTokens:   refreshTokens,
		Audit:           audit,
		ConsentGrants:   consentGrants,
		Clients:         clients,
		Cache:           cache,
		Settings:        settings,
	})
	return h, users, refreshTokens, audit, consentGrants, clients, user, db
}

func newUserTestApp(t *testing.T, h *UserHandler, userID string) *fiber.App {
	t.Helper()

	app := fiber.New()
	app.Use(RequestIDMiddleware())
	app.Use(AuditContextMiddleware())
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Put("/api/users/me", h.UpdateMe)
	app.Put("/api/users/me/password", h.ChangePassword)
	app.Delete("/api/sessions/:id", h.RevokeSession)
	app.Delete("/api/sessions", h.RevokeAllSessions)
	app.Post("/api/users/me/set-password", h.SetPassword)
	app.Get("/api/users/me/authorized-apps", h.ListAuthorizedApps)
	app.Delete("/api/users/me/authorized-apps/:id", h.RevokeAuthorizedApp)
	return app
}

func TestUserHandler_UpdateMe_WritesAuditLog(t *testing.T) {
	h, _, _, _, _, _, user, db := setupUserHandler(t)
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

func TestUserHandler_RevokeSession_WritesAuditLog(t *testing.T) {
	h, _, refreshTokens, _, _, _, user, db := setupUserHandler(t)
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
	h, users, _, _, _, _, user, db := setupUserHandler(t)
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
	h, users, _, _, _, _, user, _ := setupUserHandler(t)
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
	h, users, refreshTokens, _, _, _, user, _ := setupUserHandler(t)
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
	h, users, refreshTokens, _, _, _, user, _ := setupUserHandler(t)
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
	h, _, _, _, _, _, _, _ := setupUserHandler(t)
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

func TestUserHandlerListAuthorizedApps(t *testing.T) {
	h, _, _, _, consentGrants, clients, user, db := setupUserHandler(t)
	app := newUserTestApp(t, h, user.ID)
	ctx := context.Background()

	if err := clients.Create(ctx, &model.OAuthClient{
		Name:         "Acme Docs",
		ClientID:     "client-1",
		ClientSecret: "hash",
		RedirectURIs: "https://example.com/callback",
		UserID:       "owner-1",
	}); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := consentGrants.Upsert(ctx, &model.OAuthConsentGrant{
		UserID:   user.ID,
		ClientID: "client-1",
		Scopes:   "openid profile",
	}); err != nil {
		t.Fatalf("seed current user grant: %v", err)
	}
	if err := consentGrants.Upsert(ctx, &model.OAuthConsentGrant{
		UserID:   "other-user",
		ClientID: "client-2",
		Scopes:   "openid",
	}); err != nil {
		t.Fatalf("seed other user grant: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users/me/authorized-apps", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Apps []struct {
			ClientID   string `json:"client_id"`
			ClientName string `json:"client_name"`
			Scopes     string `json:"scopes"`
		} `json:"apps"`
		Total int64 `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Total != 1 {
		t.Fatalf("expected total 1, got %d", body.Total)
	}
	if len(body.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(body.Apps))
	}
	if body.Apps[0].ClientName != "Acme Docs" {
		t.Fatalf("expected client name Acme Docs, got %q", body.Apps[0].ClientName)
	}
	if body.Apps[0].ClientID != "client-1" {
		t.Fatalf("expected client id client-1, got %q", body.Apps[0].ClientID)
	}
	if body.Apps[0].Scopes != "openid profile" {
		t.Fatalf("expected scopes openid profile, got %q", body.Apps[0].Scopes)
	}

	assertNoUserAuditLogByAction(t, db, user.ID, model.AuditOIDCConsentGrantRevoke)
}

func TestUserHandlerRevokeAuthorizedAppDeletesGrantAndWritesAudit(t *testing.T) {
	h, _, _, _, consentGrants, _, user, db := setupUserHandler(t)
	app := newUserTestApp(t, h, user.ID)
	ctx := context.Background()

	if err := consentGrants.Upsert(ctx, &model.OAuthConsentGrant{
		UserID:   user.ID,
		ClientID: "client-1",
		Scopes:   "openid profile",
	}); err != nil {
		t.Fatalf("seed grant: %v", err)
	}
	grant, err := consentGrants.GetByUserAndClient(ctx, user.ID, "client-1")
	if err != nil {
		t.Fatalf("load grant: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/users/me/authorized-apps/"+grant.ID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if _, err := consentGrants.GetByUserAndClient(ctx, user.ID, "client-1"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected deleted grant to be missing, got %v", err)
	}

	assertUserAuditLogByAction(t, db, user.ID, model.AuditOIDCConsentGrantRevoke, "oauth_consent_grant")
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

func assertNoUserAuditLogByAction(t *testing.T, db *gorm.DB, userID, action string) {
	t.Helper()

	var auditLog model.AuditLog
	err := db.WithContext(context.Background()).
		Where("user_id = ? AND action = ?", userID, action).
		First(&auditLog).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected no audit log for action %s, got err=%v record=%+v", action, err, auditLog)
	}
}
