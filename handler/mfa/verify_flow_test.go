package mfahandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
	oidcstore "github.com/ysicing/go-template/store/oidc"

	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

func TestMFAVerify_OIDCFlowReturnsRedirect(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()
	oidcStorage, err := oidcstore.NewOIDCStorage(ctx, db, cache, userStore, clientStore, "/login", "", 0, 0, 0)
	if err != nil {
		t.Fatalf("new oidc storage: %v", err)
	}
	mfaStore := store.NewMFAStore(db, "")
	auditStore := store.NewAuditLogStore(db)
	refreshStore := store.NewAPIRefreshTokenStore(db)

	password := "Password123!abcd"
	user := createLocalUser(t, db, "mfa-verify-user", "mfa-verify@example.com", password)
	secret := "JBSWY3DPEHPK3PXP"
	if err := mfaStore.Upsert(ctx, &model.MFAConfig{UserID: user.ID, TOTPSecret: secret, TOTPEnabled: true}); err != nil {
		t.Fatalf("upsert mfa config: %v", err)
	}

	authReq, err := oidcStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-1",
		RedirectURI:  "https://example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	mfaToken := "mfa-token-oidc"
	if err := cache.Set(ctx, "mfa_pending:"+mfaToken, user.ID, 5*time.Minute); err != nil {
		t.Fatalf("set mfa pending: %v", err)
	}
	if err := cache.Set(ctx, "mfa_pending_ctx:"+mfaToken, "oidc:"+authReq.GetID(), 5*time.Minute); err != nil {
		t.Fatalf("set mfa pending context: %v", err)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}

	h := NewMFAHandler(MFADeps{
		Users:         userStore,
		MFA:           mfaStore,
		Audit:         auditStore,
		RefreshTokens: refreshStore,
		Cache:         cache,
		OIDC:          oidcStorage,
		TokenConfig: handlercommon.TokenConfig{
			Secret:        "test-secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})
	app := fiber.New()
	app.Post("/api/auth/mfa/verify", h.Verify)

	payload, _ := json.Marshal(map[string]string{
		"mfa_token": mfaToken,
		"code":      code,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expectedRedirect := "/authorize/callback?id=" + authReq.GetID()
	if body["redirect"] != expectedRedirect {
		t.Fatalf("expected redirect %q, got %#v", expectedRedirect, body["redirect"])
	}
	if _, hasAccessToken := body["access_token"]; hasAccessToken {
		t.Fatalf("expected no local access_token for oidc mfa flow, got %#v", body["access_token"])
	}

	updatedReq, err := oidcStorage.AuthRequestByID(ctx, authReq.GetID())
	if err != nil {
		t.Fatalf("get auth request by id: %v", err)
	}
	if !updatedReq.Done() {
		t.Fatal("expected auth request to be completed after MFA verification")
	}
	if updatedReq.GetSubject() != user.ID {
		t.Fatalf("expected auth request subject %q, got %q", user.ID, updatedReq.GetSubject())
	}

	if _, err := cache.Get(ctx, "mfa_pending:"+mfaToken); err != store.ErrCacheMiss {
		t.Fatalf("expected mfa_pending key deleted, got err=%v", err)
	}
	if _, err := cache.Get(ctx, "mfa_pending_ctx:"+mfaToken); err != store.ErrCacheMiss {
		t.Fatalf("expected mfa_pending_ctx key deleted, got err=%v", err)
	}
}

func TestMFAVerify_OIDCConsentPromptRedirectsToConsentPage(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()
	oidcStorage, err := oidcstore.NewOIDCStorage(ctx, db, cache, userStore, clientStore, "/login", "", 0, 0, 0)
	if err != nil {
		t.Fatalf("new oidc storage: %v", err)
	}
	mfaStore := store.NewMFAStore(db, "")
	auditStore := store.NewAuditLogStore(db)
	refreshStore := store.NewAPIRefreshTokenStore(db)

	password := "Password123!abcd"
	user := createLocalUser(t, db, "mfa-consent-user", "mfa-consent@example.com", password)
	secret := "JBSWY3DPEHPK3PXP"
	if err := mfaStore.Upsert(ctx, &model.MFAConfig{UserID: user.ID, TOTPSecret: secret, TOTPEnabled: true}); err != nil {
		t.Fatalf("upsert mfa config: %v", err)
	}

	authReq, err := oidcStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-1",
		RedirectURI:  "https://example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
		Prompt:       oidc.SpaceDelimitedArray{oidc.PromptConsent},
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	mfaToken := "mfa-token-oidc-consent"
	if err := cache.Set(ctx, "mfa_pending:"+mfaToken, user.ID, 5*time.Minute); err != nil {
		t.Fatalf("set mfa pending: %v", err)
	}
	if err := cache.Set(ctx, "mfa_pending_ctx:"+mfaToken, "oidc:"+authReq.GetID(), 5*time.Minute); err != nil {
		t.Fatalf("set mfa pending context: %v", err)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}

	h := NewMFAHandler(MFADeps{
		Users:         userStore,
		MFA:           mfaStore,
		Audit:         auditStore,
		RefreshTokens: refreshStore,
		Cache:         cache,
		OIDC:          oidcStorage,
		TokenConfig: handlercommon.TokenConfig{
			Secret:        "test-secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})
	app := fiber.New()
	app.Post("/api/auth/mfa/verify", h.Verify)

	payload, _ := json.Marshal(map[string]string{
		"mfa_token": mfaToken,
		"code":      code,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expectedRedirect := "/consent?id=" + authReq.GetID()
	if body["redirect"] != expectedRedirect {
		t.Fatalf("expected redirect %q, got %#v", expectedRedirect, body["redirect"])
	}
}

func TestMFAVerify_OIDCRequireConsentWithoutGrantRedirectsToConsentPage(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	grantStore := store.NewOAuthConsentGrantStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()
	oidcStorage, err := oidcstore.NewOIDCStorage(ctx, db, cache, userStore, clientStore, "/login", "", 0, 0, 0)
	if err != nil {
		t.Fatalf("new oidc storage: %v", err)
	}
	mfaStore := store.NewMFAStore(db, "")
	auditStore := store.NewAuditLogStore(db)
	refreshStore := store.NewAPIRefreshTokenStore(db)

	password := "Password123!abcd"
	user := createLocalUser(t, db, "mfa-grant-user", "mfa-grant@example.com", password)
	secret := "JBSWY3DPEHPK3PXP"
	if err := mfaStore.Upsert(ctx, &model.MFAConfig{UserID: user.ID, TOTPSecret: secret, TOTPEnabled: true}); err != nil {
		t.Fatalf("upsert mfa config: %v", err)
	}

	client := &model.OAuthClient{
		Name:           "Require Consent Client",
		ClientID:       "client-mfa-consent-required",
		RedirectURIs:   "https://example.com/callback",
		GrantTypes:     "authorization_code",
		Scopes:         "openid profile",
		RequireConsent: true,
	}
	if err := client.SetSecret("client-secret"); err != nil {
		t.Fatalf("set client secret: %v", err)
	}
	if err := clientStore.Create(ctx, client); err != nil {
		t.Fatalf("create client: %v", err)
	}

	authReq, err := oidcStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-mfa-consent-required",
		RedirectURI:  "https://example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	mfaToken := "mfa-token-oidc-grant"
	if err := cache.Set(ctx, "mfa_pending:"+mfaToken, user.ID, 5*time.Minute); err != nil {
		t.Fatalf("set mfa pending: %v", err)
	}
	if err := cache.Set(ctx, "mfa_pending_ctx:"+mfaToken, "oidc:"+authReq.GetID(), 5*time.Minute); err != nil {
		t.Fatalf("set mfa pending context: %v", err)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}

	h := NewMFAHandler(MFADeps{
		Users:         userStore,
		MFA:           mfaStore,
		Audit:         auditStore,
		RefreshTokens: refreshStore,
		Cache:         cache,
		OIDC:          oidcStorage,
		Clients:       clientStore,
		ConsentGrants: grantStore,
		TokenConfig: handlercommon.TokenConfig{
			Secret:        "test-secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})
	app := fiber.New()
	app.Post("/api/auth/mfa/verify", h.Verify)

	payload, _ := json.Marshal(map[string]string{
		"mfa_token": mfaToken,
		"code":      code,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expectedRedirect := "/consent?id=" + authReq.GetID()
	if body["redirect"] != expectedRedirect {
		t.Fatalf("expected redirect %q, got %#v", expectedRedirect, body["redirect"])
	}
}

func TestMFAVerify_OIDCRememberedGrantSkipsConsent(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	userStore := store.NewUserStore(db)
	clientStore := store.NewOAuthClientStore(db)
	grantStore := store.NewOAuthConsentGrantStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()
	oidcStorage, err := oidcstore.NewOIDCStorage(ctx, db, cache, userStore, clientStore, "/login", "", 0, 0, 0)
	if err != nil {
		t.Fatalf("new oidc storage: %v", err)
	}
	mfaStore := store.NewMFAStore(db, "")
	auditStore := store.NewAuditLogStore(db)
	refreshStore := store.NewAPIRefreshTokenStore(db)

	password := "Password123!abcd"
	user := createLocalUser(t, db, "mfa-grant-skip-user", "mfa-grant-skip@example.com", password)
	secret := "JBSWY3DPEHPK3PXP"
	if err := mfaStore.Upsert(ctx, &model.MFAConfig{UserID: user.ID, TOTPSecret: secret, TOTPEnabled: true}); err != nil {
		t.Fatalf("upsert mfa config: %v", err)
	}

	client := &model.OAuthClient{
		Name:           "Remembered Grant Client",
		ClientID:       "client-mfa-consent-skip",
		RedirectURIs:   "https://example.com/callback",
		GrantTypes:     "authorization_code",
		Scopes:         "openid profile",
		RequireConsent: true,
	}
	if err := client.SetSecret("client-secret"); err != nil {
		t.Fatalf("set client secret: %v", err)
	}
	if err := clientStore.Create(ctx, client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := grantStore.Upsert(ctx, &model.OAuthConsentGrant{
		UserID:   user.ID,
		ClientID: client.ClientID,
		Scopes:   "openid profile",
	}); err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	authReq, err := oidcStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-mfa-consent-skip",
		RedirectURI:  "https://example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	mfaToken := "mfa-token-oidc-grant-skip"
	if err := cache.Set(ctx, "mfa_pending:"+mfaToken, user.ID, 5*time.Minute); err != nil {
		t.Fatalf("set mfa pending: %v", err)
	}
	if err := cache.Set(ctx, "mfa_pending_ctx:"+mfaToken, "oidc:"+authReq.GetID(), 5*time.Minute); err != nil {
		t.Fatalf("set mfa pending context: %v", err)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("generate totp code: %v", err)
	}

	h := NewMFAHandler(MFADeps{
		Users:         userStore,
		MFA:           mfaStore,
		Audit:         auditStore,
		RefreshTokens: refreshStore,
		Cache:         cache,
		OIDC:          oidcStorage,
		Clients:       clientStore,
		ConsentGrants: grantStore,
		TokenConfig: handlercommon.TokenConfig{
			Secret:        "test-secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})
	app := fiber.New()
	app.Post("/api/auth/mfa/verify", h.Verify)

	payload, _ := json.Marshal(map[string]string{
		"mfa_token": mfaToken,
		"code":      code,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/mfa/verify", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	expectedRedirect := "/authorize/callback?id=" + authReq.GetID()
	if body["redirect"] != expectedRedirect {
		t.Fatalf("expected redirect %q, got %#v", expectedRedirect, body["redirect"])
	}
}
