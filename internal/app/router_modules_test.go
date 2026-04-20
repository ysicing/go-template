package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

func testRouteDeps(t *testing.T) *Deps {
	t.Helper()

	cfg := DefaultConfig()
	cfg.Database.Driver = "sqlite"
	cfg.Database.DSN = testSQLiteDSN(t)
	cfg.Security.OIDCSecret = "test-oidc-secret"

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
		RedirectURIs: "https://example.com/callback",
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
	deps.OIDCHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	client := &model.OAuthClient{
		Name:         "Machine Client",
		ClientID:     "client-token-route",
		GrantTypes:   "client_credentials",
		Scopes:       "openid profile",
		RedirectURIs: "https://example.com/callback",
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

func TestSetupRoutesOAuthTokenDelegatesNonClientCredentials(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	deps.OIDCHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"delegated":true}`))
	})

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
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected fallback status 202, got %d", resp.StatusCode)
	}
}

func TestSetupRoutesOIDCIntrospectAndRevokeClientToken(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	log := zerolog.New(io.Discard)
	provider := initOIDCProvider(deps.Config, deps, &log)
	deps.OIDCHandler = provider

	SetupRoutes(app, deps)
	mountOIDCHandler(app, provider)

	client := &model.OAuthClient{
		Name:         "Lifecycle Client",
		ClientID:     "client-lifecycle",
		GrantTypes:   "client_credentials",
		Scopes:       "read write",
		RedirectURIs: "https://example.com/callback",
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

	var logs []model.AuditLog
	if err := deps.DB.WithContext(context.Background()).
		Where("action = ? AND client_id = ?", "oauth_token_revoke", client.ClientID).
		Find(&logs).Error; err != nil {
		t.Fatalf("query revoke audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 revoke audit log, got %d", len(logs))
	}
}

func TestSetupRoutesOIDCUserinfoRejectsClientToken(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	log := zerolog.New(io.Discard)
	provider := initOIDCProvider(deps.Config, deps, &log)
	deps.OIDCHandler = provider

	SetupRoutes(app, deps)
	mountOIDCHandler(app, provider)

	client := &model.OAuthClient{
		Name:         "Userinfo Boundary Client",
		ClientID:     "client-userinfo-boundary",
		GrantTypes:   "client_credentials",
		Scopes:       "read",
		RedirectURIs: "https://example.com/callback",
	}
	secret := "userinfo-secret"
	if err := client.SetSecret(secret); err != nil {
		t.Fatalf("set client secret: %v", err)
	}
	if err := deps.ClientStore.Create(context.Background(), client); err != nil {
		t.Fatalf("create oauth client: %v", err)
	}

	token := issueClientCredentialsAccessToken(t, app, client.ClientID, secret)

	req := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET /oauth/userinfo: %v", err)
	}
	if resp.StatusCode == http.StatusOK {
		t.Fatal("expected client-principal token to be rejected by /oauth/userinfo")
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

func TestAuthConfigReturnsTemplateFlags(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	SetupRoutes(app, deps)

	ctx := context.Background()
	user := &model.User{
		Username:   "brand-owner",
		Email:      "brand-owner@example.com",
		Provider:   "local",
		ProviderID: "brand-owner",
		InviteCode: "INV-brand-owner",
	}
	if err := deps.DB.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := deps.ClientStore.Create(ctx, &model.OAuthClient{
		Name:         "Template Portal",
		ClientID:     "client-template",
		ClientSecret: "hash",
		RedirectURIs: "https://portal.example.com/callback",
		UserID:       user.ID,
	}); err != nil {
		t.Fatalf("create client: %v", err)
	}

	authReq, err := deps.OIDCStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-template",
		RedirectURI:  "https://portal.example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/config?id="+authReq.GetID(), nil)
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

func TestOIDCConsentContextReturnsClientAndScopes(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	SetupRoutes(app, deps)

	ctx := context.Background()
	user := &model.User{
		Username:   "consent-owner",
		Email:      "consent-owner@example.com",
		Provider:   "local",
		ProviderID: "consent-owner",
		InviteCode: "INV-consent-owner",
	}
	if err := deps.DB.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := deps.ClientStore.Create(ctx, &model.OAuthClient{
		Name:         "Template Docs",
		ClientID:     "client-template-consent",
		ClientSecret: "hash",
		RedirectURIs: "https://docs.example.com/callback",
		UserID:       user.ID,
	}); err != nil {
		t.Fatalf("create client: %v", err)
	}

	authReq, err := deps.OIDCStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-template-consent",
		RedirectURI:  "https://docs.example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile", "email"},
		ResponseType: oidc.ResponseTypeCode,
		Prompt:       oidc.SpaceDelimitedArray{oidc.PromptConsent},
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/consent?id="+authReq.GetID(), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("get consent context: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Client struct {
			Name string `json:"name"`
		} `json:"client"`
		Scopes []string `json:"scopes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode consent context: %v", err)
	}
	if body.Client.Name != "Template Docs" {
		t.Fatalf("expected client name Template Docs, got %q", body.Client.Name)
	}
	if len(body.Scopes) != 3 {
		t.Fatalf("expected 3 scopes, got %d", len(body.Scopes))
	}
}

func TestOIDCConsentApproveWritesAuditAndCompletesRequest(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	SetupRoutes(app, deps)

	ctx := context.Background()
	user := &model.User{
		Username:   "approve-owner",
		Email:      "approve-owner@example.com",
		Provider:   "local",
		ProviderID: "approve-owner",
		InviteCode: "INV-approve-owner",
	}
	if err := deps.DB.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := deps.ClientStore.Create(ctx, &model.OAuthClient{
		Name:         "Acme Docs",
		ClientID:     "client-acme-approve",
		ClientSecret: "hash",
		RedirectURIs: "https://docs.example.com/callback",
		UserID:       user.ID,
	}); err != nil {
		t.Fatalf("create client: %v", err)
	}

	authReq, err := deps.OIDCStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-acme-approve",
		RedirectURI:  "https://docs.example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
		Prompt:       oidc.SpaceDelimitedArray{oidc.PromptConsent},
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}
	if err := deps.OIDCStorage.AssignAuthRequestUser(context.Background(), authReq.GetID(), user.ID); err != nil {
		t.Fatalf("assign auth request user: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{"id": authReq.GetID()})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oidc/consent/approve", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("approve consent: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	updatedReq, err := deps.OIDCStorage.AuthRequestByID(ctx, authReq.GetID())
	if err != nil {
		t.Fatalf("load updated auth request: %v", err)
	}
	if !updatedReq.Done() {
		t.Fatal("expected auth request to be completed after consent approve")
	}

	audits, total, err := deps.AuditLogStore.ListAuditLogsPaged(ctx, store.AuditLogFilter{Action: model.AuditOIDCConsentApprove}, 1, 10)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if total != 1 || len(audits) != 1 {
		t.Fatalf("expected one consent approve audit log, got total=%d len=%d", total, len(audits))
	}
	if audits[0].ResourceID != authReq.GetID() {
		t.Fatalf("expected audit resource id %q, got %q", authReq.GetID(), audits[0].ResourceID)
	}
}

func TestOIDCConsentDenyWritesAuditAndDeletesRequest(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	SetupRoutes(app, deps)

	ctx := context.Background()
	user := &model.User{
		Username:   "deny-owner",
		Email:      "deny-owner@example.com",
		Provider:   "local",
		ProviderID: "deny-owner",
		InviteCode: "INV-deny-owner",
	}
	if err := deps.DB.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := deps.ClientStore.Create(ctx, &model.OAuthClient{
		Name:         "Acme Docs",
		ClientID:     "client-acme-deny",
		ClientSecret: "hash",
		RedirectURIs: "https://docs.example.com/callback",
		UserID:       user.ID,
	}); err != nil {
		t.Fatalf("create client: %v", err)
	}

	authReq, err := deps.OIDCStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-acme-deny",
		RedirectURI:  "https://docs.example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
		Prompt:       oidc.SpaceDelimitedArray{oidc.PromptConsent},
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}
	if err := deps.OIDCStorage.AssignAuthRequestUser(context.Background(), authReq.GetID(), user.ID); err != nil {
		t.Fatalf("assign auth request user: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{"id": authReq.GetID()})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oidc/consent/deny", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("deny consent: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if _, err := deps.OIDCStorage.AuthRequestByID(ctx, authReq.GetID()); err == nil {
		t.Fatal("expected auth request to be deleted after consent deny")
	}

	audits, total, err := deps.AuditLogStore.ListAuditLogsPaged(ctx, store.AuditLogFilter{Action: model.AuditOIDCConsentDeny}, 1, 10)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if total != 1 || len(audits) != 1 {
		t.Fatalf("expected one consent deny audit log, got total=%d len=%d", total, len(audits))
	}
	if audits[0].ResourceID != authReq.GetID() {
		t.Fatalf("expected audit resource id %q, got %q", authReq.GetID(), audits[0].ResourceID)
	}
}
