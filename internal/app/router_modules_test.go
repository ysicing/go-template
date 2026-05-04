package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ysicing/go-template/model"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

func testRouteDeps(t *testing.T) *Deps {
	t.Helper()

	cfg := DefaultConfig()
	cfg.Database.Driver = "sqlite"
	cfg.Database.DSN = testSQLiteDSN(t)

	log := zerolog.New(io.Discard)
	db, cache := initDBAndCache(context.Background(), cfg, &log)
	t.Cleanup(func() {
		_ = cache.Close()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	deps := initDeps(context.Background(), db, cache, cfg, &log)
	deps.Config = cfg
	return deps
}

func TestTestRouteDeps_IsolatesDatabasePerTestHelper(t *testing.T) {
	first := testRouteDeps(t)
	second := testRouteDeps(t)

	client := &model.OAuthClient{
		Name:         "Isolation Client",
		ClientID:     "client-isolation-check",
		GrantTypes:   "client_credentials",
		Scopes:       "read",
		RedirectURIs: "",
	}
	secret := "isolation-secret"
	if err := client.SetSecret(secret); err != nil {
		t.Fatalf("set client secret: %v", err)
	}
	if err := first.ClientStore.Create(context.Background(), client); err != nil {
		t.Fatalf("create oauth client in first deps: %v", err)
	}

	got, err := second.ClientStore.GetByClientID(context.Background(), client.ClientID)
	if err == nil && got != nil {
		t.Fatalf("expected second deps database to be isolated, but found client %q", client.ClientID)
	}
}

func TestSetupRoutesRegistersTemplateEndpoints(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)

	SetupRoutes(app, deps)

	for _, tc := range []struct {
		method  string
		path    string
		want404 bool
	}{
		{method: http.MethodPost, path: "/api/auth/login"},
		{method: http.MethodGet, path: "/api/apps", want404: true},
		{method: http.MethodGet, path: "/api/points"},
		{method: http.MethodGet, path: "/api/admin/users"},
		{method: http.MethodGet, path: "/api/admin/clients"},
		{method: http.MethodGet, path: "/api/admin/settings"},
		{method: http.MethodGet, path: "/api/organizations/test", want404: true},
		{method: http.MethodGet, path: "/api/workspace-plans", want404: true},
		{method: http.MethodGet, path: "/api/webhooks", want404: true},
		{method: http.MethodGet, path: "/api/monitoring/alerts", want404: true},
		{method: http.MethodGet, path: "/api/quotes/random", want404: true},
		{method: http.MethodGet, path: "/api/admin/quotes", want404: true},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if tc.want404 && resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected route %s %s to be removed, got %d", tc.method, tc.path, resp.StatusCode)
		}
		if !tc.want404 && resp.StatusCode == http.StatusNotFound {
			t.Fatalf("expected route %s %s to be registered, got 404", tc.method, tc.path)
		}
	}
}

func TestSetupRoutesRegistersOAuthTokenClientCredentialsEndpoint(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)

	client := &model.OAuthClient{
		Name:         "Machine Client",
		ClientID:     "client-token-route",
		GrantTypes:   "client_credentials",
		Scopes:       "read write",
		RedirectURIs: "",
	}
	secret := "route-secret"
	if err := client.SetSecret(secret); err != nil {
		t.Fatalf("set client secret: %v", err)
	}
	if err := deps.ClientStore.Create(context.Background(), client); err != nil {
		t.Fatalf("create oauth client: %v", err)
	}

	SetupRoutes(app, deps)

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {client.ClientID},
		"client_secret": {secret},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST /oauth/token: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from client credentials route, got %d", resp.StatusCode)
	}
}

func TestSetupRoutesOAuthTokenRejectsUnsupportedGrantType(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)

	SetupRoutes(app, deps)

	form := url.Values{
		"grant_type": {"authorization_code"},
		"code":       {"code-1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST /oauth/token: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetupRoutesOAuthIntrospectAndRevokeClientToken(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)

	SetupRoutes(app, deps)

	client := &model.OAuthClient{
		Name:         "Lifecycle Client",
		ClientID:     "client-lifecycle",
		GrantTypes:   "client_credentials",
		Scopes:       "read write",
		RedirectURIs: "",
	}
	secret := "lifecycle-secret"
	if err := client.SetSecret(secret); err != nil {
		t.Fatalf("set client secret: %v", err)
	}
	if err := deps.ClientStore.Create(context.Background(), client); err != nil {
		t.Fatalf("create oauth client: %v", err)
	}

	token := issueClientCredentialsAccessToken(t, app, client.ClientID, secret)

	introspect := introspectOAuthToken(t, app, client.ClientID, secret, token)
	if active, _ := introspect["active"].(bool); !active {
		t.Fatalf("expected active introspection response, got %v", introspect["active"])
	}
	if got, _ := introspect["sub"].(string); got != client.ClientID {
		t.Fatalf("expected sub=%q, got %q", client.ClientID, got)
	}
	if got, _ := introspect["client_id"].(string); got != client.ClientID {
		t.Fatalf("expected client_id=%q, got %q", client.ClientID, got)
	}

	revokeForm := url.Values{"token": {token}}
	revokeReq := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(revokeForm.Encode()))
	revokeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	revokeReq.SetBasicAuth(client.ClientID, secret)
	revokeResp, err := app.Test(revokeReq)
	if err != nil {
		t.Fatalf("POST /oauth/revoke: %v", err)
	}
	if revokeResp.StatusCode != http.StatusOK {
		t.Fatalf("expected revoke status 200, got %d", revokeResp.StatusCode)
	}

	introspect = introspectOAuthToken(t, app, client.ClientID, secret, token)
	if active, _ := introspect["active"].(bool); active {
		t.Fatalf("expected inactive introspection response after revoke, got %v", introspect["active"])
	}
}

func TestAuthConfigReturnsTemplateFlags(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	SetupRoutes(app, deps)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("get auth config: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		RegisterEnabled       bool   `json:"register_enabled"`
		TurnstileSiteKey      string `json:"turnstile_site_key"`
		EmailVerificationMode bool   `json:"email_verification_enabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode auth config: %v", err)
	}
	if !body.RegisterEnabled {
		t.Fatal("expected registration to stay enabled by default")
	}
	if body.TurnstileSiteKey != "" {
		t.Fatalf("expected empty turnstile site key by default, got %q", body.TurnstileSiteKey)
	}
}

func issueClientCredentialsAccessToken(t *testing.T, app *fiber.App, clientID, clientSecret string) string {
	t.Helper()

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST /oauth/token: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected token status 200, got %d", resp.StatusCode)
	}

	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if body.AccessToken == "" {
		t.Fatal("expected access_token in response")
	}
	return body.AccessToken
}

func introspectOAuthToken(t *testing.T, app *fiber.App, clientID, clientSecret, token string) map[string]any {
	t.Helper()

	form := url.Values{"token": {token}}
	req := httptest.NewRequest(http.MethodPost, "/oauth/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST /oauth/introspect: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected introspect status 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode introspect response: %v", err)
	}
	return body
}
