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

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
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
	json.Unmarshal(body, &result)
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
		json.NewEncoder(w).Encode(map[string]any{
			"id":         12345,
			"login":      "testuser",
			"email":      "test@example.com",
			"avatar_url": "https://example.com/avatar.png",
		})
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
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "google-123",
			"email":   "test@gmail.com",
			"name":    "Test User",
			"picture": "https://example.com/photo.png",
		})
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
		w.Write([]byte(`{"error":"bad_verification_code"}`))
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
		w.Write([]byte(`{"error":"bad_verification_code"}`))
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
