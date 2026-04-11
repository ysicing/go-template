package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
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
		DB:             db,
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
