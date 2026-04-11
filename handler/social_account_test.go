package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

type socialAccountNotifierStub struct {
	provider   string
	providerID string
	calls      int
}

func (s *socialAccountNotifierStub) NotifySocialAccountUnlinked(_ context.Context, provider, providerID string) {
	s.provider = provider
	s.providerID = providerID
	s.calls++
}

func (s *socialAccountNotifierStub) NotifySocialAccountLinked(_ context.Context, provider, providerID string) {
	s.provider = provider
	s.providerID = providerID
	s.calls++
}

func TestSocialAccountHandler_ListMySocialAccounts_IncludesProviderID(t *testing.T) {
	db := setupTestDB(t)
	socialAccounts := store.NewSocialAccountStore(db)
	users := store.NewUserStore(db)
	audit := store.NewAuditLogStore(db)
	user := createLocalUser(t, db, "social-user", "social-user@example.com", "Password123!abcd")

	if err := socialAccounts.Create(t.Context(), &model.SocialAccount{
		UserID:      user.ID,
		Provider:    model.SocialProviderGitHub,
		ProviderID:  "gh-123456789",
		Username:    "alice",
		DisplayName: "Alice",
	}); err != nil {
		t.Fatalf("seed social account: %v", err)
	}

	h := NewSocialAccountHandler(socialAccounts, users, audit, nil)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	})
	app.Get("/api/users/me/social-accounts", h.ListMySocialAccounts)

	req := httptest.NewRequest(http.MethodGet, "/api/users/me/social-accounts", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body []struct {
		Provider    string `json:"provider"`
		ProviderID  string `json:"provider_id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 social account, got %d", len(body))
	}
	if body[0].Provider != model.SocialProviderGitHub {
		t.Fatalf("expected provider github, got %q", body[0].Provider)
	}
	if body[0].ProviderID != "gh-123456789" {
		t.Fatalf("expected provider_id gh-123456789, got %q", body[0].ProviderID)
	}
	if body[0].Username != "alice" {
		t.Fatalf("expected username alice, got %q", body[0].Username)
	}
	if body[0].DisplayName != "Alice" {
		t.Fatalf("expected display name Alice, got %q", body[0].DisplayName)
	}
}

func TestSocialAccountHandler_UnlinkSocialAccount_NotifiesLinkedProvider(t *testing.T) {
	db := setupTestDB(t)
	socialAccounts := store.NewSocialAccountStore(db)
	users := store.NewUserStore(db)
	audit := store.NewAuditLogStore(db)
	notifier := &socialAccountNotifierStub{}
	user := createLocalUser(t, db, "unlink-github-user", "unlink-github@example.com", "Password123!abcd")

	account := &model.SocialAccount{
		UserID:      user.ID,
		Provider:    model.SocialProviderGitHub,
		ProviderID:  "gh-99887766",
		Username:    "unlink-user",
		DisplayName: "Unlink User",
	}
	if err := socialAccounts.Create(t.Context(), account); err != nil {
		t.Fatalf("seed social account: %v", err)
	}

	h := NewSocialAccountHandler(socialAccounts, users, audit, notifier)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	})
	app.Delete("/api/users/me/social-accounts/:id", h.UnlinkSocialAccount)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/me/social-accounts/"+account.ID, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if notifier.calls != 1 {
		t.Fatalf("expected notifier called once, got %d", notifier.calls)
	}
	if notifier.provider != model.SocialProviderGitHub {
		t.Fatalf("expected provider github, got %q", notifier.provider)
	}
	if notifier.providerID != "gh-99887766" {
		t.Fatalf("expected provider id gh-99887766, got %q", notifier.providerID)
	}

	if _, err := socialAccounts.GetByProviderAndID(t.Context(), model.SocialProviderGitHub, "gh-99887766"); err == nil {
		t.Fatal("expected social account deleted")
	}
}
