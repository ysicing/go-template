package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

func TestAdminHandler_ListUsers_IncludesSocialProviders(t *testing.T) {
	db := setupTestDB(t)

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	auditStore := store.NewAuditLogStore(db)
	socialAccounts := store.NewSocialAccountStore(db)

	user := createLocalUser(t, db, "list-user", "list-user@example.com", "Password123!abcd")
	if err := socialAccounts.Create(t.Context(), &model.SocialAccount{
		UserID:     user.ID,
		Provider:   model.SocialProviderGoogle,
		ProviderID: "google-123456789",
	}); err != nil {
		t.Fatalf("seed google social account: %v", err)
	}
	if err := socialAccounts.Create(t.Context(), &model.SocialAccount{
		UserID:     user.ID,
		Provider:   model.SocialProviderGitHub,
		ProviderID: "gh-123",
		Email:      "list-user@example.com",
	}); err != nil {
		t.Fatalf("seed github social account: %v", err)
	}

	h := NewAdminHandler(AdminDeps{
		Users:          userStore,
		Clients:        clientStore,
		Audit:          auditStore,
		SocialAccounts: socialAccounts,
	})

	app := fiber.New()
	app.Get("/api/admin/users", h.ListUsers)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Users []struct {
			ID              string   `json:"id"`
			SocialProviders []string `json:"social_providers"`
		} `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, item := range body.Users {
		if item.ID != user.ID {
			continue
		}
		found = true
		if !slices.Equal(item.SocialProviders, []string{model.SocialProviderGitHub, model.SocialProviderGoogle}) {
			t.Fatalf("expected sorted social providers [github google], got %#v", item.SocialProviders)
		}
	}
	if !found {
		t.Fatalf("expected user %q in response", user.ID)
	}
}

func TestAdminHandler_CreateUser_ReturnsSetupTokenInsteadOfPassword(t *testing.T) {
	db := setupTestDB(t)
	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	auditStore := store.NewAuditLogStore(db)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	h := NewAdminHandler(AdminDeps{
		Users:   userStore,
		Clients: clientStore,
		Audit:   auditStore,
		Cache:   cache,
	})

	app := fiber.New()
	app.Post("/api/admin/users", func(c fiber.Ctx) error {
		c.Locals("user_id", "admin-user")
		return h.CreateUser(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(`{"username":"new-admin-user","email":"new-admin@example.com","is_admin":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, exists := body["password"]; exists {
		t.Fatalf("expected plaintext password to be omitted, got %#v", body["password"])
	}
	token, _ := body["password_setup_token"].(string)
	if token == "" {
		t.Fatal("expected password_setup_token in response")
	}

	created, err := userStore.GetByEmail(context.Background(), "new-admin@example.com")
	if err != nil {
		t.Fatalf("get created user: %v", err)
	}
	if created.PasswordHash != "" {
		t.Fatal("expected created user to require password setup")
	}
	if !created.IsAdmin {
		t.Fatal("expected created user to be admin")
	}
}
