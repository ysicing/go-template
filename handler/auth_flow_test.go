package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
	oidcstore "github.com/ysicing/go-template/store/oidc"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

func TestOIDCLoginSubmit_MFARequired(t *testing.T) {
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

	password := "Password123!abcd"
	user := createLocalUser(t, db, "oidc-mfa-user", "oidc-mfa@example.com", password)
	if err := mfaStore.Upsert(ctx, &model.MFAConfig{UserID: user.ID, TOTPSecret: "JBSWY3DPEHPK3PXP", TOTPEnabled: true}); err != nil {
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

	h := NewOIDCLoginHandler(oidcStorage, clientStore, nil, userStore, mfaStore, auditStore, cache)
	app := fiber.New()
	app.Post("/api/auth/oidc-login", h.LoginSubmit)

	payload, _ := json.Marshal(map[string]string{
		"id":       authReq.GetID(),
		"username": user.Username,
		"password": password,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oidc-login", bytes.NewReader(payload))
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
	if required, ok := body["mfa_required"].(bool); !ok || !required {
		t.Fatalf("expected mfa_required=true, got %#v", body["mfa_required"])
	}
	mfaToken, ok := body["mfa_token"].(string)
	if !ok || mfaToken == "" {
		t.Fatalf("expected mfa_token in response, got %#v", body["mfa_token"])
	}

	updatedReq, err := oidcStorage.AuthRequestByID(ctx, authReq.GetID())
	if err != nil {
		t.Fatalf("get auth request by id: %v", err)
	}
	if updatedReq.Done() {
		t.Fatal("expected auth request to remain pending before MFA verification")
	}

	pendingUserID, err := cache.Get(ctx, "mfa_pending:"+mfaToken)
	if err != nil {
		t.Fatalf("get mfa pending user id: %v", err)
	}
	if pendingUserID != user.ID {
		t.Fatalf("expected mfa pending user %q, got %q", user.ID, pendingUserID)
	}

	pendingCtx, err := cache.Get(ctx, "mfa_pending_ctx:"+mfaToken)
	if err != nil {
		t.Fatalf("get mfa pending context: %v", err)
	}
	expectedCtx := "oidc:" + authReq.GetID()
	if pendingCtx != expectedCtx {
		t.Fatalf("expected mfa pending context %q, got %q", expectedCtx, pendingCtx)
	}
}

func TestOIDCLoginSubmit_ConsentPromptRedirectsToConsentPage(t *testing.T) {
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

	password := "Password123!abcd"
	user := createLocalUser(t, db, "oidc-consent-user", "oidc-consent@example.com", password)

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

	h := NewOIDCLoginHandler(oidcStorage, clientStore, nil, userStore, mfaStore, auditStore, cache)
	app := fiber.New()
	app.Post("/api/auth/oidc-login", h.LoginSubmit)

	payload, _ := json.Marshal(map[string]string{
		"id":       authReq.GetID(),
		"username": user.Username,
		"password": password,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oidc-login", bytes.NewReader(payload))
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

func TestOIDCLoginSubmit_RequireConsentWithoutGrantRedirectsToConsentPage(t *testing.T) {
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

	password := "Password123!abcd"
	user := createLocalUser(t, db, "oidc-grant-user", "oidc-grant@example.com", password)
	if err := clientStore.Create(ctx, &model.OAuthClient{
		Name:           "Consent App",
		ClientID:       "client-consent-required",
		ClientSecret:   "hash",
		RedirectURIs:   "https://example.com/callback",
		UserID:         user.ID,
		RequireConsent: true,
	}); err != nil {
		t.Fatalf("create client: %v", err)
	}

	authReq, err := oidcStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-consent-required",
		RedirectURI:  "https://example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	h := NewOIDCLoginHandler(oidcStorage, clientStore, nil, userStore, mfaStore, auditStore, cache)
	app := fiber.New()
	app.Post("/api/auth/oidc-login", h.LoginSubmit)

	payload, _ := json.Marshal(map[string]string{
		"id":       authReq.GetID(),
		"username": user.Username,
		"password": password,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oidc-login", bytes.NewReader(payload))
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

func TestOIDCConsentApproveStoresGrantAndNextLoginSkipsConsent(t *testing.T) {
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

	password := "Password123!abcd"
	user := createLocalUser(t, db, "oidc-repeat-user", "oidc-repeat@example.com", password)
	if err := clientStore.Create(ctx, &model.OAuthClient{
		Name:           "Consent App",
		ClientID:       "client-repeat-consent",
		ClientSecret:   "hash",
		RedirectURIs:   "https://example.com/callback",
		UserID:         user.ID,
		RequireConsent: true,
	}); err != nil {
		t.Fatalf("create client: %v", err)
	}

	h := NewOIDCLoginHandler(oidcStorage, clientStore, grantStore, userStore, mfaStore, auditStore, cache)
	app := fiber.New()
	app.Post("/api/auth/oidc-login", h.LoginSubmit)
	app.Post("/api/auth/oidc/consent/approve", h.ConsentApprove)

	firstReq, err := oidcStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-repeat-consent",
		RedirectURI:  "https://example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
	}, "")
	if err != nil {
		t.Fatalf("create first auth request: %v", err)
	}

	firstPayload, _ := json.Marshal(map[string]string{
		"id":       firstReq.GetID(),
		"username": user.Username,
		"password": password,
	})
	firstLoginReq := httptest.NewRequest(http.MethodPost, "/api/auth/oidc-login", bytes.NewReader(firstPayload))
	firstLoginReq.Header.Set("Content-Type", "application/json")
	firstLoginResp, err := app.Test(firstLoginReq)
	if err != nil {
		t.Fatalf("first login request failed: %v", err)
	}
	if firstLoginResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected first login 200, got %d", firstLoginResp.StatusCode)
	}

	approvePayload, _ := json.Marshal(map[string]string{"id": firstReq.GetID()})
	approveReq := httptest.NewRequest(http.MethodPost, "/api/auth/oidc/consent/approve", bytes.NewReader(approvePayload))
	approveReq.Header.Set("Content-Type", "application/json")
	approveResp, err := app.Test(approveReq)
	if err != nil {
		t.Fatalf("approve request failed: %v", err)
	}
	if approveResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected approve 200, got %d", approveResp.StatusCode)
	}

	grant, err := grantStore.GetByUserAndClient(ctx, user.ID, "client-repeat-consent")
	if err != nil {
		t.Fatalf("load stored grant: %v", err)
	}
	if grant.Scopes != "openid profile" {
		t.Fatalf("expected stored scopes %q, got %q", "openid profile", grant.Scopes)
	}

	secondReq, err := oidcStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-repeat-consent",
		RedirectURI:  "https://example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
	}, "")
	if err != nil {
		t.Fatalf("create second auth request: %v", err)
	}

	secondPayload, _ := json.Marshal(map[string]string{
		"id":       secondReq.GetID(),
		"username": user.Username,
		"password": password,
	})
	secondLoginReq := httptest.NewRequest(http.MethodPost, "/api/auth/oidc-login", bytes.NewReader(secondPayload))
	secondLoginReq.Header.Set("Content-Type", "application/json")
	secondLoginResp, err := app.Test(secondLoginReq)
	if err != nil {
		t.Fatalf("second login request failed: %v", err)
	}
	if secondLoginResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected second login 200, got %d", secondLoginResp.StatusCode)
	}

	var secondBody map[string]any
	if err := json.NewDecoder(secondLoginResp.Body).Decode(&secondBody); err != nil {
		t.Fatalf("decode second login response: %v", err)
	}
	expectedRedirect := "/authorize/callback?id=" + secondReq.GetID()
	if secondBody["redirect"] != expectedRedirect {
		t.Fatalf("expected redirect %q, got %#v", expectedRedirect, secondBody["redirect"])
	}
}

func TestOIDCLoginSubmit_RevokedGrantRequiresConsentAgain(t *testing.T) {
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

	password := "Password123!abcd"
	user := createLocalUser(t, db, "oidc-revoke-user", "oidc-revoke@example.com", password)
	if err := clientStore.Create(ctx, &model.OAuthClient{
		Name:           "Consent App",
		ClientID:       "client-revoked-consent",
		ClientSecret:   "hash",
		RedirectURIs:   "https://example.com/callback",
		UserID:         user.ID,
		RequireConsent: true,
	}); err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := grantStore.Upsert(ctx, &model.OAuthConsentGrant{
		UserID:   user.ID,
		ClientID: "client-revoked-consent",
		Scopes:   "openid profile",
	}); err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	grant, err := grantStore.GetByUserAndClient(ctx, user.ID, "client-revoked-consent")
	if err != nil {
		t.Fatalf("load grant: %v", err)
	}
	if err := grantStore.DeleteByIDAndUserID(ctx, grant.ID, user.ID); err != nil {
		t.Fatalf("delete grant: %v", err)
	}

	h := NewOIDCLoginHandler(oidcStorage, clientStore, grantStore, userStore, mfaStore, auditStore, cache)
	app := fiber.New()
	app.Post("/api/auth/oidc-login", h.LoginSubmit)

	authReq, err := oidcStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-revoked-consent",
		RedirectURI:  "https://example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"id":       authReq.GetID(),
		"username": user.Username,
		"password": password,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oidc-login", bytes.NewReader(payload))
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

func TestOIDCLoginSubmit_PromptConsentOverridesRememberedGrant(t *testing.T) {
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

	password := "Password123!abcd"
	user := createLocalUser(t, db, "oidc-prompt-user", "oidc-prompt@example.com", password)
	if err := clientStore.Create(ctx, &model.OAuthClient{
		Name:           "Consent App",
		ClientID:       "client-prompt-consent",
		ClientSecret:   "hash",
		RedirectURIs:   "https://example.com/callback",
		UserID:         user.ID,
		RequireConsent: true,
	}); err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := grantStore.Upsert(ctx, &model.OAuthConsentGrant{
		UserID:   user.ID,
		ClientID: "client-prompt-consent",
		Scopes:   "openid profile email",
	}); err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	h := NewOIDCLoginHandler(oidcStorage, clientStore, grantStore, userStore, mfaStore, auditStore, cache)
	app := fiber.New()
	app.Post("/api/auth/oidc-login", h.LoginSubmit)

	authReq, err := oidcStorage.CreateAuthRequest(ctx, &oidc.AuthRequest{
		ClientID:     "client-prompt-consent",
		RedirectURI:  "https://example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "profile"},
		ResponseType: oidc.ResponseTypeCode,
		Prompt:       oidc.SpaceDelimitedArray{oidc.PromptConsent},
	}, "")
	if err != nil {
		t.Fatalf("create auth request: %v", err)
	}

	payload, _ := json.Marshal(map[string]string{
		"id":       authReq.GetID(),
		"username": user.Username,
		"password": password,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oidc-login", bytes.NewReader(payload))
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

func TestAuthLogout_RevokesOnlyCurrentRefreshToken(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	userStore := store.NewUserStore(db)
	refreshStore := store.NewAPIRefreshTokenStore(db)
	mfaStore := store.NewMFAStore(db, "")
	auditStore := store.NewAuditLogStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	user := createLocalUser(t, db, "logout-user", "logout@example.com", "Password123!abcd")

	currentToken := "refresh-token-current"
	otherToken := "refresh-token-other"
	currentHash := store.HashToken(currentToken)
	otherHash := store.HashToken(otherToken)

	if err := refreshStore.Create(ctx, &model.APIRefreshToken{UserID: user.ID, TokenHash: currentHash, ExpiresAt: time.Now().Add(24 * time.Hour)}); err != nil {
		t.Fatalf("create current refresh token: %v", err)
	}
	if err := refreshStore.Create(ctx, &model.APIRefreshToken{UserID: user.ID, TokenHash: otherHash, ExpiresAt: time.Now().Add(24 * time.Hour)}); err != nil {
		t.Fatalf("create other refresh token: %v", err)
	}

	h := NewAuthHandler(AuthDeps{
		Users:         userStore,
		RefreshTokens: refreshStore,
		MFA:           mfaStore,
		Audit:         auditStore,
		Cache:         cache,
		TokenConfig: TokenConfig{
			Secret:        "test-secret",
			Issuer:        "id",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})
	app := fiber.New()
	app.Post("/api/auth/logout", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return h.Logout(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Cookie", "refresh_token="+currentToken)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	if _, err := refreshStore.GetByTokenHash(ctx, currentHash); err == nil {
		t.Fatal("expected current refresh token to be revoked")
	}
	if _, err := refreshStore.GetByTokenHash(ctx, otherHash); err != nil {
		t.Fatalf("expected other refresh token to remain valid: %v", err)
	}
}

func TestGenerateAccessToken_IncludesTokenVersionClaim(t *testing.T) {
	secret := "jwt-secret"
	issuer := "id"
	token, err := GenerateAccessToken("user-1", false, []string{model.PermissionAdminUsersRead}, 3, secret, issuer, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("parse token: %v", err)
	}
	if claims.TokenVersion != 3 {
		t.Fatalf("expected token_version=3, got %d", claims.TokenVersion)
	}
}

func TestJWTMiddleware_RejectsQueryToken(t *testing.T) {
	secret := "jwt-secret"
	issuer := "id"
	token, err := GenerateAccessToken("user-1", false, nil, 1, secret, issuer, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}

	app := fiber.New()
	app.Get("/protected", JWTMiddleware(secret, issuer), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	queryReq := httptest.NewRequest(http.MethodGet, "/protected?token="+token, nil)
	queryResp, err := app.Test(queryReq)
	if err != nil {
		t.Fatalf("query request failed: %v", err)
	}
	if queryResp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected query token request to be unauthorized, got %d", queryResp.StatusCode)
	}

	headerReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	headerReq.Header.Set("Authorization", "Bearer "+token)
	headerResp, err := app.Test(headerReq)
	if err != nil {
		t.Fatalf("header request failed: %v", err)
	}
	if headerResp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected bearer header request to pass, got %d", headerResp.StatusCode)
	}
}

func TestJWTMiddleware_LegacyTokenWithoutTokenVersionDefaultsToOne(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	secret := "jwt-secret"
	issuer := "id"
	u := createLocalUser(t, db, "legacy-token", "legacy-token@example.com", "Password123!abcd")
	u.TokenVersion = 1
	u.SetPermissions([]string{model.PermissionAdminUsersRead})
	if err := db.Save(u).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	legacyClaims := jwt.MapClaims{
		"user_id":     u.ID,
		"is_admin":    false,
		"permissions": []string{model.PermissionAdminUsersRead},
		"token_type":  "access",
		"iss":         issuer,
		"sub":         u.ID,
		"aud":         []string{"id-api"},
		"exp":         time.Now().Add(time.Hour).Unix(),
		"iat":         time.Now().Unix(),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, legacyClaims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}

	app := fiber.New()
	app.Get("/api/admin/users", JWTMiddleware(secret, issuer), RequirePermission(users, cache, model.PermissionAdminUsersRead), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestJWTMiddleware_LegacyTokenWithoutTokenVersionSetsLocalVersionOne(t *testing.T) {
	secret := "jwt-secret"
	issuer := "id"

	legacyClaims := jwt.MapClaims{
		"user_id":     "legacy-user",
		"is_admin":    false,
		"permissions": []string{model.PermissionAdminUsersRead},
		"token_type":  "access",
		"iss":         issuer,
		"sub":         "legacy-user",
		"aud":         []string{"id-api"},
		"exp":         time.Now().Add(time.Hour).Unix(),
		"iat":         time.Now().Unix(),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, legacyClaims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign legacy token: %v", err)
	}

	app := fiber.New()
	app.Get("/protected", JWTMiddleware(secret, issuer), func(c fiber.Ctx) error {
		tokenVersion, _ := c.Locals("token_version").(int64)
		if tokenVersion != 1 {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "unexpected token version"})
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestTokenVersionMiddleware_DeniesMismatchedTokenOnProtectedRoute(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	secret := "jwt-secret"
	issuer := "id"
	user := createLocalUser(t, db, "tv-user", "tv-user@example.com", "Password123!abcd")
	user.TokenVersion = 2
	if err := users.Update(context.Background(), user); err != nil {
		t.Fatalf("update user token version: %v", err)
	}

	oldToken, err := GenerateAccessToken(user.ID, false, nil, 1, secret, issuer, time.Hour)
	if err != nil {
		t.Fatalf("generate old token: %v", err)
	}
	newToken, err := GenerateAccessToken(user.ID, false, nil, 2, secret, issuer, time.Hour)
	if err != nil {
		t.Fatalf("generate new token: %v", err)
	}

	app := fiber.New()
	app.Get("/protected", JWTMiddleware(secret, issuer), TokenVersionMiddleware(users, cache), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	oldReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	oldReq.Header.Set("Authorization", "Bearer "+oldToken)
	oldResp, err := app.Test(oldReq)
	if err != nil {
		t.Fatalf("old token request failed: %v", err)
	}
	if oldResp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected old token denied with 401, got %d", oldResp.StatusCode)
	}

	newReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	newReq.Header.Set("Authorization", "Bearer "+newToken)
	newResp, err := app.Test(newReq)
	if err != nil {
		t.Fatalf("new token request failed: %v", err)
	}
	if newResp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected new token accepted with 204, got %d", newResp.StatusCode)
	}
}

func TestRequirePermission_DeniesWhenTokenVersionMismatch(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	target := createLocalUser(t, db, "perm-ver", "perm-ver@example.com", "Password123!abcd")
	target.TokenVersion = 2
	target.SetPermissions([]string{model.PermissionAdminUsersRead})
	if err := db.Save(target).Error; err != nil {
		t.Fatalf("save target user: %v", err)
	}

	app := fiber.New()
	app.Get("/protected", func(c fiber.Ctx) error {
		c.Locals("user_id", target.ID)
		c.Locals("is_admin", false)
		c.Locals("token_version", int64(1))
		c.Locals("permissions", []string{model.PermissionAdminUsersRead})
		return RequirePermission(users, cache, model.PermissionAdminUsersRead)(c)
	}, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/protected", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestRequirePermission_UsesClaimsPermissions(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	target := createLocalUser(t, db, "perm-claims", "perm-claims@example.com", "Password123!abcd")

	app := fiber.New()
	app.Get("/protected", func(c fiber.Ctx) error {
		c.Locals("user_id", target.ID)
		c.Locals("is_admin", false)
		c.Locals("permissions", []string{model.PermissionAdminUsersRead})
		return RequirePermission(users, cache, model.PermissionAdminUsersRead)(c)
	}, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected status 204, got %d", resp.StatusCode)
	}
}

func TestRequirePermission_DeniesWithoutPermission(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	target := createLocalUser(t, db, "perm-deny", "perm-deny@example.com", "Password123!abcd")

	app := fiber.New()
	app.Get("/protected", func(c fiber.Ctx) error {
		c.Locals("user_id", target.ID)
		c.Locals("is_admin", false)
		c.Locals("permissions", []string{model.PermissionAdminUsersRead})
		return RequirePermission(users, cache, model.PermissionAdminSettingsWrite)(c)
	}, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestRequirePermission_LoadsPermissionFromDB(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	target := createLocalUser(t, db, "perm-db", "perm-db@example.com", "Password123!abcd")
	target.SetPermissions([]string{model.PermissionAdminSettingsRead})
	if err := db.Save(target).Error; err != nil {
		t.Fatalf("save target user: %v", err)
	}

	app := fiber.New()
	app.Get("/protected", func(c fiber.Ctx) error {
		c.Locals("user_id", target.ID)
		c.Locals("is_admin", false)
		c.Locals("permissions", []string{})
		return RequirePermission(users, cache, model.PermissionAdminSettingsRead)(c)
	}, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected status 204, got %d", resp.StatusCode)
	}

	cacheKey := "perm_check:" + target.ID + ":" + model.PermissionAdminSettingsRead
	cached, err := cache.Get(context.Background(), cacheKey)
	if err != nil {
		t.Fatalf("expected cache key to be set: %v", err)
	}
	if cached != "1" {
		t.Fatalf("expected cache value 1, got %q", cached)
	}
}

func TestRequirePermission_DeniesAndCachesNegativeResult(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	target := createLocalUser(t, db, "perm-negative", "perm-negative@example.com", "Password123!abcd")

	app := fiber.New()
	app.Get("/protected", func(c fiber.Ctx) error {
		c.Locals("user_id", target.ID)
		c.Locals("is_admin", false)
		c.Locals("permissions", []string{})
		return RequirePermission(users, cache, model.PermissionAdminSettingsRead)(c)
	}, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}

	cacheKey := "perm_check:" + target.ID + ":" + model.PermissionAdminSettingsRead
	cached, err := cache.Get(context.Background(), cacheKey)
	if err != nil {
		t.Fatalf("expected cache key to be set: %v", err)
	}
	if cached != "0" {
		t.Fatalf("expected cache value 0, got %q", cached)
	}
}

func TestAdminRoutePermissionMatrix(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	defer cache.Close()

	secret := "matrix-secret"
	issuer := "id"

	user := createLocalUser(t, db, "perm-matrix", "perm-matrix@example.com", "Password123!abcd")
	user.SetPermissions([]string{model.PermissionAdminStatsRead})
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user permissions: %v", err)
	}

	app := fiber.New()
	app.Get("/api/admin/users", JWTMiddleware(secret, issuer), RequirePermission(users, cache, model.PermissionAdminUsersRead), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	app.Post("/api/admin/settings", JWTMiddleware(secret, issuer), RequirePermission(users, cache, model.PermissionAdminSettingsWrite), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	app.Get("/api/admin/stats", JWTMiddleware(secret, issuer), RequirePermission(users, cache, model.PermissionAdminStatsRead), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	cases := []struct {
		name        string
		method      string
		path        string
		isAdmin     bool
		permissions []string
		expected    int
	}{
		{
			name:        "users read permission can access users list",
			method:      http.MethodGet,
			path:        "/api/admin/users",
			permissions: []string{model.PermissionAdminUsersRead},
			expected:    fiber.StatusNoContent,
		},
		{
			name:        "users read permission cannot write settings",
			method:      http.MethodPost,
			path:        "/api/admin/settings",
			permissions: []string{model.PermissionAdminUsersRead},
			expected:    fiber.StatusForbidden,
		},
		{
			name:        "settings write permission can write settings",
			method:      http.MethodPost,
			path:        "/api/admin/settings",
			permissions: []string{model.PermissionAdminSettingsWrite},
			expected:    fiber.StatusNoContent,
		},
		{
			name:     "admin bypasses permission checks",
			method:   http.MethodPost,
			path:     "/api/admin/settings",
			isAdmin:  true,
			expected: fiber.StatusNoContent,
		},
		{
			name:     "token without claims permissions falls back to db",
			method:   http.MethodGet,
			path:     "/api/admin/stats",
			expected: fiber.StatusNoContent,
		},
		{
			name:     "token without permission denied",
			method:   http.MethodGet,
			path:     "/api/admin/users",
			expected: fiber.StatusForbidden,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateAccessToken(user.ID, tt.isAdmin, tt.permissions, user.TokenVersion, secret, issuer, time.Hour)
			if err != nil {
				t.Fatalf("generate access token: %v", err)
			}

			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.StatusCode != tt.expected {
				t.Fatalf("expected status %d, got %d", tt.expected, resp.StatusCode)
			}
		})
	}
}
