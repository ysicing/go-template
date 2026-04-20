package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

func seedClientCredentialsOAuthClient(t *testing.T, clients *store.OAuthClientStore, grantTypes, scopes string) (*model.OAuthClient, string) {
	t.Helper()

	client := &model.OAuthClient{
		Name:         "Machine Client",
		ClientID:     "client-1",
		GrantTypes:   grantTypes,
		Scopes:       scopes,
		RedirectURIs: "https://example.com/callback",
	}
	secret := "super-secret"
	if err := client.SetSecret(secret); err != nil {
		t.Fatalf("set client secret: %v", err)
	}
	if err := clients.Create(context.Background(), client); err != nil {
		t.Fatalf("create oauth client: %v", err)
	}
	return client, secret
}

func TestClientCredentialsHandlerTokenHandlesClientCredentialsGrant(t *testing.T) {
	db := setupTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	client, secret := seedClientCredentialsOAuthClient(t, clients, "client_credentials", "openid profile")

	fallbackCalled := false
	h := NewClientCredentialsHandler(
		service.NewClientCredentialsService(service.ClientCredentialsServiceDeps{
			DB:      db,
			Clients: clients,
			Audit:   audit,
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fallbackCalled = true
			w.WriteHeader(http.StatusTeapot)
		}),
	)

	app := fiber.New()
	app.Post("/oauth/token", h.Token)

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {client.ClientID},
		"client_secret": {secret},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if fallbackCalled {
		t.Fatal("expected client_credentials requests to avoid fallback handler")
	}

	var body struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if body.AccessToken == "" {
		t.Fatal("expected access token in response")
	}
	if body.TokenType != "Bearer" {
		t.Fatalf("expected token_type=Bearer, got %q", body.TokenType)
	}
	if body.ExpiresIn <= 0 {
		t.Fatalf("expected positive expires_in, got %d", body.ExpiresIn)
	}
}

func TestClientCredentialsHandlerTokenDelegatesOtherGrantTypes(t *testing.T) {
	db := setupTestDB(t)

	fallbackCalled := false
	h := NewClientCredentialsHandler(
		service.NewClientCredentialsService(service.ClientCredentialsServiceDeps{
			DB:      db,
			Clients: store.NewOAuthClientStore(db),
			Audit:   store.NewAuditLogStore(db),
		}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fallbackCalled = true
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte("fallback"))
		}),
	)

	app := fiber.New()
	app.Post("/oauth/token", h.Token)

	form := url.Values{
		"grant_type": {"authorization_code"},
		"code":       {"code-1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("expected fallback status 418, got %d", resp.StatusCode)
	}
	if !fallbackCalled {
		t.Fatal("expected non-client_credentials requests to use fallback handler")
	}
}
