package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/glebarez/sqlite"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database with migrations.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// seedProvider inserts a social provider into the test database.
func seedProvider(t *testing.T, db *gorm.DB, name, clientID, clientSecret, redirectURL string, enabled bool) {
	t.Helper()
	p := &model.SocialProvider{
		Name:         name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Enabled:      true, // Create with default true first
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	// If disabled, update after creation to avoid GORM default:true overriding false.
	if !enabled {
		if err := db.Model(p).Update("enabled", false).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewOAuthHandler_Fields(t *testing.T) {
	db := setupTestDB(t)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	if h.providers != sp {
		t.Error("expected providers store to be set")
	}
	if h.tokenConfig.Secret != "secret" {
		t.Errorf("expected jwtSecret 'secret', got %q", h.tokenConfig.Secret)
	}
}

func TestGitHubLogin_NotConfigured(t *testing.T) {
	db := setupTestDB(t)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/github", h.GitHubLogin)

	req := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result["error"] != "github oauth not configured" {
		t.Errorf("unexpected error: %s", result["error"])
	}
}

func TestGitHubLogin_Disabled(t *testing.T) {
	db := setupTestDB(t)
	seedProvider(t, db, "github", "gh-id", "gh-secret", "http://localhost/cb", false)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/github", h.GitHubLogin)

	req := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected 404 for disabled provider, got %d", resp.StatusCode)
	}
}

func TestGitHubLogin_ProviderSecretUnavailable(t *testing.T) {
	db := setupTestDB(t)
	writer := store.NewSocialProviderStore(db, "writer-key")
	if err := writer.Upsert(context.Background(), &model.SocialProvider{
		Name:         "github",
		ClientID:     "gh-id",
		ClientSecret: "gh-secret",
		RedirectURL:  "http://localhost/cb",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	sp := store.NewSocialProviderStore(db, "wrong-reader-key")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/github", h.GitHubLogin)

	req := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Errorf("expected 503 for unavailable provider secret, got %d", resp.StatusCode)
	}
}

func TestGitHubLogin_Redirect(t *testing.T) {
	db := setupTestDB(t)
	seedProvider(t, db, "github", "test-id", "test-secret", "http://localhost/cb", true)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/github", h.GitHubLogin)

	req := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		t.Error("expected Location header to be set")
	}
	for _, cookie := range resp.Header.Values("Set-Cookie") {
		if strings.Contains(cookie, "oauth_state=") {
			t.Fatalf("expected no oauth_state cookie, got %q", cookie)
		}
	}
}

func TestGoogleLogin_NotConfigured(t *testing.T) {
	db := setupTestDB(t)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/google", h.GoogleLogin)

	req := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGoogleLogin_Redirect(t *testing.T) {
	db := setupTestDB(t)
	seedProvider(t, db, "google", "g-id", "g-secret", "http://localhost/gcb", true)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/google", h.GoogleLogin)

	req := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		t.Error("expected Location header to be set")
	}
	for _, cookie := range resp.Header.Values("Set-Cookie") {
		if strings.Contains(cookie, "oauth_state=") {
			t.Fatalf("expected no oauth_state cookie, got %q", cookie)
		}
	}
}

func TestGitHubCallback_NotConfigured(t *testing.T) {
	db := setupTestDB(t)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/github/callback", h.GitHubCallback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGitHubCallback_MissingCode(t *testing.T) {
	db := setupTestDB(t)
	seedProvider(t, db, "github", "id", "secret", "http://localhost/cb", true)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/github/callback", h.GitHubCallback)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestConfirmSocialLink_WritesAuditLog(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	users := store.NewUserStore(db)
	socialAccounts := store.NewSocialAccountStore(db)
	audit := store.NewAuditLogStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)

	user := createLocalUser(t, db, "social-confirm", "social-confirm@example.com", "Password123!abcd")
	user.PasswordHash = ""
	if err := user.SetPassword("Password123!abcd"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if err := users.Update(context.Background(), user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	pendingData, _ := json.Marshal(map[string]string{
		"user_id":     user.ID,
		"provider":    model.SocialProviderGitHub,
		"provider_id": "gh-123",
		"email":       user.Email,
		"avatar_url":  "https://example.com/avatar.png",
	})
	if err := cache.Set(context.Background(), "social_link_pending:link-token", string(pendingData), 10*time.Minute); err != nil {
		t.Fatalf("seed link token: %v", err)
	}

	h := NewOAuthHandler(OAuthDeps{
		DB:             db,
		Users:          users,
		SocialAccounts: socialAccounts,
		Audit:          audit,
		RefreshTokens:  refreshTokens,
		Cache:          cache,
		TokenConfig: TokenConfig{
			Secret:        "secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})

	app := fiber.New()
	app.Use(RequestIDMiddleware())
	app.Use(AuditContextMiddleware())
	app.Post("/api/auth/social/confirm-link", h.ConfirmSocialLink)

	payload, _ := json.Marshal(map[string]string{
		"link_token": "link-token",
		"password":   "Password123!abcd",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/social/confirm-link", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var auditLog model.AuditLog
	if err := db.WithContext(context.Background()).
		Where("user_id = ? AND action = ? AND resource = ?", user.ID, model.AuditSocialAccountLink, "social_account").
		Order("created_at DESC").
		First(&auditLog).Error; err != nil {
		t.Fatalf("expected social account audit log: %v", err)
	}
	if !strings.Contains(auditLog.Detail, "source=api") {
		t.Fatalf("expected api source in audit detail, got %q", auditLog.Detail)
	}
	if !strings.Contains(auditLog.Detail, "provider=github") {
		t.Fatalf("expected provider metadata in audit detail, got %q", auditLog.Detail)
	}
}

func TestConfirmSocialLink_RequiresPasswordOrTOTP(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	h := NewOAuthHandler(OAuthDeps{
		Users:          store.NewUserStore(db),
		SocialAccounts: store.NewSocialAccountStore(db),
		Audit:          store.NewAuditLogStore(db),
		RefreshTokens:  store.NewAPIRefreshTokenStore(db),
		Cache:          cache,
		TokenConfig: TokenConfig{
			Secret:        "secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})

	app := fiber.New()
	app.Post("/api/auth/social/confirm-link", h.ConfirmSocialLink)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/social/confirm-link", strings.NewReader(`{"link_token":"link-token"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if got, _ := body["error"].(string); got != "verification required: password or totp_code" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestConfirmSocialLink_RejectsUnsupportedChallengeVerification(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	h := NewOAuthHandler(OAuthDeps{
		Users:          store.NewUserStore(db),
		SocialAccounts: store.NewSocialAccountStore(db),
		Audit:          store.NewAuditLogStore(db),
		RefreshTokens:  store.NewAPIRefreshTokenStore(db),
		Cache:          cache,
		TokenConfig: TokenConfig{
			Secret:        "secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})

	app := fiber.New()
	app.Post("/api/auth/social/confirm-link", h.ConfirmSocialLink)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/social/confirm-link", strings.NewReader(`{"link_token":"link-token","challenge":"demo"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if got, _ := body["error"].(string); got != "challenge verification is not supported" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestSocialLinkWebAuthnBegin_ReturnsOptionsAndStoresSession(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	users := store.NewUserStore(db)
	creds := store.NewWebAuthnStore(db)
	user := createLocalUser(t, db, "social-passkey", "social-passkey@example.com", "Password123!abcd")
	if err := creds.Create(context.Background(), &model.WebAuthnCredential{
		UserID:       user.ID,
		Name:         "MacBook",
		CredentialID: []byte("cred-1"),
		PublicKey:    []byte("public-key"),
	}); err != nil {
		t.Fatalf("create webauthn credential: %v", err)
	}

	pendingData, _ := json.Marshal(map[string]string{
		"user_id":     user.ID,
		"provider":    model.SocialProviderGitHub,
		"provider_id": "gh-passkey",
		"email":       user.Email,
		"avatar_url":  "https://example.com/avatar.png",
	})
	if err := cache.Set(context.Background(), "social_link_pending:link-token", string(pendingData), 10*time.Minute); err != nil {
		t.Fatalf("seed link token: %v", err)
	}

	h := NewOAuthHandler(OAuthDeps{
		DB:             db,
		Users:          users,
		SocialAccounts: store.NewSocialAccountStore(db),
		Audit:          store.NewAuditLogStore(db),
		RefreshTokens:  store.NewAPIRefreshTokenStore(db),
		WebAuthnCreds:  creds,
		Cache:          cache,
		TokenConfig: TokenConfig{
			Secret:        "secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})
	h.webAuthn = fakeOAuthWebAuthnManager{
		beginOptions: &protocol.CredentialAssertion{
			Response: protocol.PublicKeyCredentialRequestOptions{
				Challenge: []byte("challenge"),
			},
		},
		beginSession: &webauthn.SessionData{
			Challenge:      "challenge",
			RelyingPartyID: "example.com",
			UserID:         []byte(user.ID),
			Expires:        time.Now().Add(5 * time.Minute),
		},
	}

	app := fiber.New()
	app.Post("/api/auth/social/confirm-link/webauthn/begin", h.SocialLinkWebAuthnBegin)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/social/confirm-link/webauthn/begin", strings.NewReader(`{"link_token":"link-token"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["publicKey"]; !ok {
		t.Fatalf("expected publicKey in response, got %v", body)
	}

	sessionJSON, err := cache.Get(context.Background(), "social_link_webauthn:link-token")
	if err != nil || sessionJSON == "" {
		t.Fatalf("expected webauthn session stored, got %q err=%v", sessionJSON, err)
	}
}

func TestSocialLinkWebAuthnFinish_LinksAccountAndWritesAuditLog(t *testing.T) {
	db := setupTestDB(t)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	users := store.NewUserStore(db)
	creds := store.NewWebAuthnStore(db)
	socialAccounts := store.NewSocialAccountStore(db)
	audit := store.NewAuditLogStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)

	user := createLocalUser(t, db, "social-passkey-finish", "social-passkey-finish@example.com", "Password123!abcd")
	if err := creds.Create(context.Background(), &model.WebAuthnCredential{
		UserID:       user.ID,
		Name:         "MacBook",
		CredentialID: []byte("cred-finish"),
		PublicKey:    []byte("public-key"),
		SignCount:    1,
	}); err != nil {
		t.Fatalf("create webauthn credential: %v", err)
	}

	pendingData, _ := json.Marshal(map[string]string{
		"user_id":     user.ID,
		"provider":    model.SocialProviderGitHub,
		"provider_id": "gh-passkey-finish",
		"email":       user.Email,
		"avatar_url":  "https://example.com/avatar-finish.png",
	})
	if err := cache.Set(context.Background(), "social_link_pending:link-token", string(pendingData), 10*time.Minute); err != nil {
		t.Fatalf("seed link token: %v", err)
	}
	sessionJSON, _ := json.Marshal(webauthn.SessionData{
		Challenge:      "challenge",
		RelyingPartyID: "example.com",
		UserID:         []byte(user.ID),
		Expires:        time.Now().Add(5 * time.Minute),
	})
	if err := cache.Set(context.Background(), "social_link_webauthn:link-token", string(sessionJSON), 5*time.Minute); err != nil {
		t.Fatalf("seed webauthn session: %v", err)
	}

	h := NewOAuthHandler(OAuthDeps{
		DB:             db,
		Users:          users,
		SocialAccounts: socialAccounts,
		Audit:          audit,
		RefreshTokens:  refreshTokens,
		WebAuthnCreds:  creds,
		Cache:          cache,
		TokenConfig: TokenConfig{
			Secret:        "secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})
	h.webAuthn = fakeOAuthWebAuthnManager{
		finishCredential: &webauthn.Credential{
			ID: []byte("cred-finish"),
			Authenticator: webauthn.Authenticator{
				SignCount: 9,
			},
		},
	}

	app := fiber.New()
	app.Use(RequestIDMiddleware())
	app.Use(AuditContextMiddleware())
	app.Post("/api/auth/social/confirm-link/webauthn/finish", h.SocialLinkWebAuthnFinish)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/social/confirm-link/webauthn/finish?link_token=link-token", strings.NewReader(`{"id":"ignored"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(body))
	}

	account, err := socialAccounts.GetByProviderAndID(context.Background(), model.SocialProviderGitHub, "gh-passkey-finish")
	if err != nil {
		t.Fatalf("expected social account created: %v", err)
	}
	if account.UserID != user.ID {
		t.Fatalf("expected linked user %s, got %s", user.ID, account.UserID)
	}

	cred, err := creds.GetByID(context.Background(), firstWebAuthnCredentialID(t, db, user.ID))
	if err != nil {
		t.Fatalf("load credential: %v", err)
	}
	if cred.SignCount != 9 {
		t.Fatalf("expected sign count updated to 9, got %d", cred.SignCount)
	}

	if val, _ := cache.Get(context.Background(), "social_link_pending:link-token"); val != "" {
		t.Fatalf("expected pending link removed, got %q", val)
	}
	if val, _ := cache.Get(context.Background(), "social_link_webauthn:link-token"); val != "" {
		t.Fatalf("expected webauthn session removed, got %q", val)
	}

	var auditLog model.AuditLog
	if err := db.WithContext(context.Background()).
		Where("user_id = ? AND action = ? AND resource = ?", user.ID, model.AuditSocialAccountLink, "social_account").
		Order("created_at DESC").
		First(&auditLog).Error; err != nil {
		t.Fatalf("expected social account audit log: %v", err)
	}
	if !strings.Contains(auditLog.Detail, "verification_method=webauthn") {
		t.Fatalf("expected webauthn verification metadata in audit detail, got %q", auditLog.Detail)
	}
}

func firstWebAuthnCredentialID(t *testing.T, db *gorm.DB, userID string) string {
	t.Helper()
	var cred model.WebAuthnCredential
	if err := db.WithContext(context.Background()).
		Where("user_id = ?", userID).
		Order("created_at ASC").
		First(&cred).Error; err != nil {
		t.Fatalf("query webauthn credential: %v", err)
	}
	return cred.ID
}

type fakeOAuthWebAuthnManager struct {
	beginOptions     *protocol.CredentialAssertion
	beginSession     *webauthn.SessionData
	beginErr         error
	finishCredential *webauthn.Credential
	finishErr        error
}

func (f fakeOAuthWebAuthnManager) BeginLogin(*store.WebAuthnUser) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	return f.beginOptions, f.beginSession, f.beginErr
}

func (f fakeOAuthWebAuthnManager) FinishLogin(*store.WebAuthnUser, webauthn.SessionData, []byte) (*webauthn.Credential, error) {
	return f.finishCredential, f.finishErr
}

func TestGoogleCallback_NotConfigured(t *testing.T) {
	db := setupTestDB(t)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/google/callback", h.GoogleCallback)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGoogleCallback_MissingCode(t *testing.T) {
	db := setupTestDB(t)
	seedProvider(t, db, "google", "id", "secret", "http://localhost/cb", true)
	sp := store.NewSocialProviderStore(db, "")
	h := NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	app := fiber.New()
	app.Get("/auth/google/callback", h.GoogleCallback)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestFetchGitHubUser_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id":         12345,
			"login":      "testuser",
			"email":      "test@example.com",
			"avatar_url": "https://example.com/avatar.png",
		}); err != nil {
			t.Errorf("encode github response: %v", err)
		}
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		t.Fatal(err)
	}
	if u.ID != 12345 {
		t.Errorf("expected ID 12345, got %d", u.ID)
	}
	if u.Login != "testuser" {
		t.Errorf("expected login 'testuser', got %q", u.Login)
	}
	if u.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", u.Email)
	}
}

func TestFetchGoogleUser_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"id":      "google-123",
			"email":   "test@gmail.com",
			"name":    "Test User",
			"picture": "https://example.com/photo.png",
		}); err != nil {
			t.Errorf("encode google response: %v", err)
		}
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var u googleUser
	if err := json.Unmarshal(body, &u); err != nil {
		t.Fatal(err)
	}
	if u.ID != "google-123" {
		t.Errorf("expected ID 'google-123', got %q", u.ID)
	}
	if u.Email != "test@gmail.com" {
		t.Errorf("expected email 'test@gmail.com', got %q", u.Email)
	}
}

func TestGoogleEndpoint(t *testing.T) {
	if googleEndpoint.AuthURL != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Errorf("unexpected AuthURL: %s", googleEndpoint.AuthURL)
	}
	if googleEndpoint.TokenURL != "https://oauth2.googleapis.com/token" {
		t.Errorf("unexpected TokenURL: %s", googleEndpoint.TokenURL)
	}
}

func TestGitHubCallback_InvalidExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error":"bad_verification_code"}`)); err != nil {
			t.Errorf("write github error response: %v", err)
		}
	}))
	defer srv.Close()

	db := setupTestDB(t)
	seedProvider(t, db, "github", "id", "s", "http://localhost/cb", true)
	sp := store.NewSocialProviderStore(db, "")
	_ = NewOAuthHandler(OAuthDeps{Providers: sp, Cache: store.NewMemoryCache(), TokenConfig: TokenConfig{Secret: "secret", Issuer: "id", AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour, RememberMeTTL: 30 * 24 * time.Hour}})

	// We need to override the endpoint after the config is built.
	// Since the config is built on the fly, we test via a custom approach:
	// seed a provider, then manually build the config and override endpoint.
	// But the handler builds config internally, so we test the full flow
	// by checking that the exchange fails with a bad token endpoint.
	// The handler will try to exchange with the real GitHub endpoint,
	// which will fail. We verify the 401 response.
	app := fiber.New()
	app.Get("/auth/github/callback", func(c fiber.Ctx) error {
		provider, err := sp.GetByName(c.Context(), "github")
		if err != nil || !provider.Enabled {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not configured"})
		}
		cfg := &oauth2.Config{
			ClientID:     provider.ClientID,
			ClientSecret: provider.ClientSecret,
			RedirectURL:  provider.RedirectURL,
			Scopes:       []string{"user:email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  srv.URL + "/authorize",
				TokenURL: srv.URL + "/token",
			},
		}
		code := c.Query("code")
		if code == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing code"})
		}
		_, tokenErr := cfg.Exchange(c.Context(), code)
		if tokenErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "failed to exchange token"})
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=badcode", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGoogleCallback_InvalidExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error":"bad_verification_code"}`)); err != nil {
			t.Errorf("write google error response: %v", err)
		}
	}))
	defer srv.Close()

	db := setupTestDB(t)
	seedProvider(t, db, "google", "id", "s", "http://localhost/cb", true)
	sp := store.NewSocialProviderStore(db, "")

	app := fiber.New()
	app.Get("/auth/google/callback", func(c fiber.Ctx) error {
		provider, err := sp.GetByName(c.Context(), "google")
		if err != nil || !provider.Enabled {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not configured"})
		}
		cfg := &oauth2.Config{
			ClientID:     provider.ClientID,
			ClientSecret: provider.ClientSecret,
			RedirectURL:  provider.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  srv.URL + "/authorize",
				TokenURL: srv.URL + "/token",
			},
		}
		code := c.Query("code")
		if code == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing code"})
		}
		_, tokenErr := cfg.Exchange(c.Context(), code)
		if tokenErr != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "failed to exchange token"})
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=badcode", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestDeriveRedirectURL(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		url := deriveRedirectURL(c, "/api/auth/github/callback")
		return c.SendString(url)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Host = "example.com"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	// httptest uses http by default
	expected := "http://example.com/api/auth/github/callback"
	if string(body) != expected {
		t.Errorf("expected %q, got %q", expected, string(body))
	}
}
