package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
	pointstore "github.com/ysicing/go-template/store/points"

	"github.com/gofiber/fiber/v3"
)

func setupEmailHandler(t *testing.T) (*EmailHandler, *store.UserStore, *store.SettingStore, store.Cache) {
	t.Helper()
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Keep SQLite in-memory schema visible across operations.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	points := pointstore.NewPointStore(db)
	h := NewEmailHandler(users, settings, audit, points, cache)
	return h, users, settings, cache
}

func createTestUser(t *testing.T, users *store.UserStore, email string, verified bool) *model.User {
	t.Helper()
	user := &model.User{
		Username:      strings.Split(email, "@")[0],
		Email:         email,
		Provider:      "local",
		ProviderID:    "testuser-" + email,
		InviteCode:    "INV-" + email,
		EmailVerified: verified,
	}
	_ = user.SetPassword("TestPass123!@#")
	if err := users.Create(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	return user
}

func TestSendTestEmail_SMTPNotConfigured(t *testing.T) {
	h, _, _, _ := setupEmailHandler(t)

	err := h.SendTestEmail(context.Background(), "target@example.com")
	if err == nil {
		t.Fatal("expected error when SMTP is not configured")
	}
	if err.Error() != "SMTP not configured" {
		t.Fatalf("expected SMTP not configured error, got %q", err.Error())
	}
}

func TestVerifyEmail_ValidToken(t *testing.T) {
	h, users, _, cache := setupEmailHandler(t)
	user := createTestUser(t, users, "test@example.com", false)
	ephemeral := store.NewEphemeralTokenStore(cache)

	// Set a verification token in cache
	token := "valid-test-token"
	if err := ephemeral.IssueString(context.Background(), "verify", "email", token, user.ID, emailVerifyTTL); err != nil {
		t.Fatalf("issue verification token: %v", err)
	}

	app := fiber.New()
	app.Post("/verify-email", h.VerifyEmail)

	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/verify-email", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify user is now marked as verified
	updated, _ := users.GetByID(context.Background(), user.ID)
	if !updated.EmailVerified {
		t.Error("expected EmailVerified to be true")
	}

	// Token should be deleted
	if _, err := ephemeral.LoadString(context.Background(), "verify", "email", token); err == nil {
		t.Error("expected token to be deleted from cache")
	}
}

func TestVerifyEmail_InvalidToken(t *testing.T) {
	h, _, _, _ := setupEmailHandler(t)

	app := fiber.New()
	app.Post("/verify-email", h.VerifyEmail)

	body, _ := json.Marshal(map[string]string{"token": "nonexistent-token"})
	req := httptest.NewRequest(http.MethodPost, "/verify-email", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestVerifyEmail_EmptyToken(t *testing.T) {
	h, _, _, _ := setupEmailHandler(t)

	app := fiber.New()
	app.Post("/verify-email", h.VerifyEmail)

	body, _ := json.Marshal(map[string]string{"token": ""})
	req := httptest.NewRequest(http.MethodPost, "/verify-email", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestVerifyEmail_AlreadyVerified(t *testing.T) {
	h, users, _, cache := setupEmailHandler(t)
	user := createTestUser(t, users, "verified@example.com", true)
	ephemeral := store.NewEphemeralTokenStore(cache)

	token := "already-verified-token"
	if err := ephemeral.IssueString(context.Background(), "verify", "email", token, user.ID, emailVerifyTTL); err != nil {
		t.Fatalf("issue verification token: %v", err)
	}

	app := fiber.New()
	app.Post("/verify-email", h.VerifyEmail)

	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/verify-email", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestResendVerification_AlreadyVerified(t *testing.T) {
	h, users, _, _ := setupEmailHandler(t)
	user := createTestUser(t, users, "verified@example.com", true)

	app := fiber.New()
	app.Post("/resend", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	}, h.ResendVerification)

	req := httptest.NewRequest(http.MethodPost, "/resend", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestResendVerification_RateLimit(t *testing.T) {
	h, users, _, cache := setupEmailHandler(t)
	user := createTestUser(t, users, "unverified@example.com", false)

	// Pre-set the rate limit key
	_ = cache.Set(context.Background(), emailResendKey+user.ID, "1", emailResendTTL)

	app := fiber.New()
	app.Post("/resend", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	}, h.ResendVerification)

	req := httptest.NewRequest(http.MethodPost, "/resend", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
}

func TestResendVerification_Unauthorized(t *testing.T) {
	h, _, _, _ := setupEmailHandler(t)

	app := fiber.New()
	app.Post("/resend", h.ResendVerification)

	req := httptest.NewRequest(http.MethodPost, "/resend", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestResendVerification_SMTPFailureStillConsumesRateLimit(t *testing.T) {
	h, users, settings, cache := setupEmailHandler(t)
	user := createTestUser(t, users, "send-fail@example.com", false)
	_ = settings.Set(context.Background(), store.SettingSMTPHost, "")

	app := fiber.New()
	app.Post("/resend", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	}, h.ResendVerification)

	req1 := httptest.NewRequest(http.MethodPost, "/resend", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatal(err)
	}
	if resp1.StatusCode != fiber.StatusOK {
		t.Fatalf("expected first resend to be queued with 200, got %d", resp1.StatusCode)
	}

	if _, err := cache.Get(context.Background(), emailResendKey+user.ID); err != nil {
		t.Fatal("expected resend rate-limit key to exist after email is queued")
	}

	req2 := httptest.NewRequest(http.MethodPost, "/resend", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected second resend to be rate limited with 429, got %d", resp2.StatusCode)
	}
}

func TestRegister_WithEmailVerification_SendQueuedWhenSMTPMissing(t *testing.T) {
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)

	// Enable email verification and keep SMTP unconfigured.
	_ = settings.Set(context.Background(), store.SettingEmailVerificationEnabled, "true")

	emailH := NewEmailHandler(users, settings, audit, pointstore.NewPointStore(db), cache)
	authH := NewAuthHandler(AuthDeps{
		Users:         users,
		RefreshTokens: refreshTokens,
		Audit:         audit,
		Cache:         cache,
		Settings:      settings,
		EmailHandler:  emailH,
		TokenConfig: TokenConfig{
			Secret:        "test-secret",
			Issuer:        "test-issuer",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})

	app := fiber.New()
	app.Post("/register", authH.Register)

	body, _ := json.Marshal(map[string]string{
		"username": "newuser",
		"email":    "new@example.com",
		"password": "StrongPass123!@#",
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	respBody, _ := readBody(resp)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	_ = json.Unmarshal(respBody, &result)
	if result["email_verification_required"] != true {
		t.Fatalf("expected email_verification_required=true, got %#v", result["email_verification_required"])
	}
	userResp, ok := result["user"].(map[string]any)
	if !ok {
		t.Fatal("expected user object in register response")
	}
	if _, exists := userResp["invite_code"]; exists {
		t.Fatalf("expected invite_code to be omitted from register response, got %#v", userResp["invite_code"])
	}
	if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("expected no session cookies when email verification is required, got %#v", got)
	}
	created, err := users.GetByEmail(context.Background(), "new@example.com")
	if err != nil {
		t.Fatalf("get registered user: %v", err)
	}
	tokens, err := refreshTokens.ListByUserID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("list refresh tokens: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected no refresh tokens before email verification, got %d", len(tokens))
	}
}

func TestAuthSetupPassword_ConsumesSetupToken(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)

	user := &model.User{
		Username:   "setup-user",
		Email:      "setup@example.com",
		Provider:   "local",
		ProviderID: "setup-user",
	}
	if err := users.Create(context.Background(), user); err != nil {
		t.Fatalf("create setup user: %v", err)
	}

	token := "setup-token"
	ephemeral := store.NewEphemeralTokenStore(cache)
	if err := ephemeral.IssueString(context.Background(), "password_setup", "user", token, user.ID, time.Hour); err != nil {
		t.Fatalf("issue setup token: %v", err)
	}

	authH := NewAuthHandler(AuthDeps{
		Users:         users,
		RefreshTokens: refreshTokens,
		Audit:         audit,
		Cache:         cache,
		Settings:      settings,
		TokenConfig: TokenConfig{
			Secret:        "test-secret",
			Issuer:        "test-issuer",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})

	app := fiber.New()
	app.Post("/setup-password", authH.SetupPassword)

	req := httptest.NewRequest(http.MethodPost, "/setup-password", strings.NewReader(`{"token":"setup-token","password":"StrongPass123!@#"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		respBody, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	updated, err := users.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if updated.PasswordHash == "" || !updated.CheckPassword("StrongPass123!@#") {
		t.Fatal("expected password to be set from setup token")
	}
	if _, err := ephemeral.LoadString(context.Background(), "password_setup", "user", token); err == nil {
		t.Fatal("expected setup token to be consumed")
	}
}

func TestRegister_WithInviteCode_BindsInviterAndInviteIP(t *testing.T) {
	defer trustAll(t)()

	db := setupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)

	inviter := createTestUser(t, users, "inviter@example.com", true)
	inviter.InviteCode = "INVITER-CODE"
	if err := users.Update(context.Background(), inviter); err != nil {
		t.Fatalf("update inviter invite code: %v", err)
	}

	emailH := NewEmailHandler(users, settings, audit, pointstore.NewPointStore(db), cache)
	authH := NewAuthHandler(AuthDeps{
		Users:         users,
		RefreshTokens: refreshTokens,
		Audit:         audit,
		Cache:         cache,
		Settings:      settings,
		EmailHandler:  emailH,
		TokenConfig: TokenConfig{
			Secret:        "test-secret",
			Issuer:        "test-issuer",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})

	app := fiber.New()
	app.Post("/register", func(c fiber.Ctx) error {
		c.Request().Header.Set("X-Real-IP", "198.51.100.24")
		return authH.Register(c)
	})

	body, _ := json.Marshal(map[string]string{
		"username":    "invitee-user",
		"email":       "invitee@example.com",
		"password":    "StrongPass123!@#",
		"invite_code": inviter.InviteCode,
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		respBody, _ := readBody(resp)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	invitee, err := users.GetByEmail(context.Background(), "invitee@example.com")
	if err != nil {
		t.Fatalf("get invitee by email: %v", err)
	}
	if invitee.InvitedByUserID != inviter.ID {
		t.Fatalf("expected invited_by_user_id=%q, got %q", inviter.ID, invitee.InvitedByUserID)
	}
	if invitee.InviteIP != "198.51.100.24" {
		t.Fatalf("expected invite_ip=198.51.100.24, got %q", invitee.InviteIP)
	}
	if invitee.InviteCode == "" {
		t.Fatal("expected invitee invite_code to be generated")
	}
}

func TestRegister_WithInvalidInviteCode_ReturnsBadRequest(t *testing.T) {
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)

	emailH := NewEmailHandler(users, settings, audit, pointstore.NewPointStore(db), cache)
	authH := NewAuthHandler(AuthDeps{
		Users:         users,
		RefreshTokens: refreshTokens,
		Audit:         audit,
		Cache:         cache,
		Settings:      settings,
		EmailHandler:  emailH,
		TokenConfig: TokenConfig{
			Secret:        "test-secret",
			Issuer:        "test-issuer",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})

	app := fiber.New()
	app.Post("/register", authH.Register)

	body, _ := json.Marshal(map[string]string{
		"username":    "bad-invite-user",
		"email":       "bad-invite@example.com",
		"password":    "StrongPass123!@#",
		"invite_code": "NOT-EXIST",
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		respBody, _ := readBody(resp)
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := readBody(resp)
	var result map[string]any
	_ = json.Unmarshal(respBody, &result)
	if result["error"] != "invalid invite code" {
		t.Fatalf("expected error 'invalid invite code', got %#v", result["error"])
	}
}

func TestRegister_WithoutInviteCode_StillSucceeds(t *testing.T) {
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)

	emailH := NewEmailHandler(users, settings, audit, pointstore.NewPointStore(db), cache)
	authH := NewAuthHandler(AuthDeps{
		Users:         users,
		RefreshTokens: refreshTokens,
		Audit:         audit,
		Cache:         cache,
		Settings:      settings,
		EmailHandler:  emailH,
		TokenConfig: TokenConfig{
			Secret:        "test-secret",
			Issuer:        "test-issuer",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})

	app := fiber.New()
	app.Post("/register", authH.Register)

	body, _ := json.Marshal(map[string]string{
		"username": "plain-user",
		"email":    "plain@example.com",
		"password": "StrongPass123!@#",
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		respBody, _ := readBody(resp)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	user, err := users.GetByEmail(context.Background(), "plain@example.com")
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	if user.InvitedByUserID != "" {
		t.Fatalf("expected invited_by_user_id empty, got %q", user.InvitedByUserID)
	}
}

func TestRegister_AllowsWeakPasswordWhenPolicyDisabled(t *testing.T) {
	db := setupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	refreshTokens := store.NewAPIRefreshTokenStore(db)

	// Explicitly disable password policy for this test.
	_ = settings.Set(context.Background(), store.SettingPasswordPolicyEnabled, "false")

	authH := NewAuthHandler(AuthDeps{
		Users:         users,
		RefreshTokens: refreshTokens,
		Audit:         audit,
		Cache:         cache,
		Settings:      settings,
		TokenConfig: TokenConfig{
			Secret:        "test-secret",
			Issuer:        "test-issuer",
			AccessTTL:     time.Hour,
			RefreshTTL:    24 * time.Hour,
			RememberMeTTL: 30 * 24 * time.Hour,
		},
	})

	app := fiber.New()
	app.Post("/register", authH.Register)

	body, _ := json.Marshal(map[string]string{
		"username": "weakpassuser",
		"email":    "weak@example.com",
		"password": "weak",
	})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		respBody, _ := readBody(resp)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestVerifyEmail_InviteRewardGrantedWhenIPsDiffer(t *testing.T) {
	defer trustAll(t)()

	db := setupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	h := NewEmailHandler(users, settings, audit, pointstore.NewPointStore(db), cache)
	ephemeral := store.NewEphemeralTokenStore(cache)
	ctx := context.Background()

	inviter := createTestUser(t, users, "reward-inviter@example.com", true)
	invitee := createTestUser(t, users, "reward-invitee@example.com", false)
	invitee.InvitedByUserID = inviter.ID
	invitee.InviteIP = "198.51.100.10"
	if err := users.Update(ctx, invitee); err != nil {
		t.Fatalf("update invitee relation: %v", err)
	}

	if err := settings.Set(ctx, store.SettingInviteRewardEnabled, "true"); err != nil {
		t.Fatalf("set invite_reward_enabled: %v", err)
	}
	if err := settings.Set(ctx, store.SettingInviteRewardMin, "2"); err != nil {
		t.Fatalf("set invite_reward_min: %v", err)
	}
	if err := settings.Set(ctx, store.SettingInviteRewardMax, "2"); err != nil {
		t.Fatalf("set invite_reward_max: %v", err)
	}

	token := "invite-reward-token"
	if err := ephemeral.IssueString(ctx, "verify", "email", token, invitee.ID, emailVerifyTTL); err != nil {
		t.Fatalf("issue verification token: %v", err)
	}

	app := fiber.New()
	app.Post("/verify-email", func(c fiber.Ctx) error {
		c.Request().Header.Set("X-Real-IP", "198.51.100.11")
		return h.VerifyEmail(c)
	})

	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/verify-email", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		respBody, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	pointStore := pointstore.NewPointStore(db)
	points, err := pointStore.GetOrCreateUserPoints(ctx, inviter.ID)
	if err != nil {
		t.Fatalf("get inviter points: %v", err)
	}
	if points.FreeBalance != 2 {
		t.Fatalf("expected inviter free balance=2, got %d", points.FreeBalance)
	}

	txns, total, err := pointStore.ListTransactions(ctx, inviter.ID, 1, 20)
	if err != nil {
		t.Fatalf("list inviter transactions: %v", err)
	}
	if total != 1 || len(txns) != 1 {
		t.Fatalf("expected one invite reward transaction, total=%d len=%d", total, len(txns))
	}
	if txns[0].Kind != model.PointKindInviteReward {
		t.Fatalf("expected txn kind %q, got %q", model.PointKindInviteReward, txns[0].Kind)
	}
	if txns[0].Amount != 2 {
		t.Fatalf("expected txn amount 2, got %d", txns[0].Amount)
	}

	var auditLog model.AuditLog
	if err := db.WithContext(ctx).Where("action = ?", model.AuditInviteRewardGranted).First(&auditLog).Error; err != nil {
		t.Fatalf("expected invite_reward_granted audit log: %v", err)
	}
	if auditLog.UserID != inviter.ID {
		t.Fatalf("expected audit user_id=%q, got %q", inviter.ID, auditLog.UserID)
	}
}

func TestVerifyEmail_InviteRewardSkippedWhenIPsSame(t *testing.T) {
	defer trustAll(t)()

	db := setupTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	audit := store.NewAuditLogStore(db)
	h := NewEmailHandler(users, settings, audit, pointstore.NewPointStore(db), cache)
	ephemeral := store.NewEphemeralTokenStore(cache)
	ctx := context.Background()

	inviter := createTestUser(t, users, "skip-inviter@example.com", true)
	invitee := createTestUser(t, users, "skip-invitee@example.com", false)
	invitee.InvitedByUserID = inviter.ID
	invitee.InviteIP = "198.51.100.33"
	if err := users.Update(ctx, invitee); err != nil {
		t.Fatalf("update invitee relation: %v", err)
	}

	if err := settings.Set(ctx, store.SettingInviteRewardEnabled, "true"); err != nil {
		t.Fatalf("set invite_reward_enabled: %v", err)
	}
	if err := settings.Set(ctx, store.SettingInviteRewardMin, "3"); err != nil {
		t.Fatalf("set invite_reward_min: %v", err)
	}
	if err := settings.Set(ctx, store.SettingInviteRewardMax, "3"); err != nil {
		t.Fatalf("set invite_reward_max: %v", err)
	}

	token := "invite-reward-skip-token"
	if err := ephemeral.IssueString(ctx, "verify", "email", token, invitee.ID, emailVerifyTTL); err != nil {
		t.Fatalf("issue verification token: %v", err)
	}

	app := fiber.New()
	app.Post("/verify-email", func(c fiber.Ctx) error {
		c.Request().Header.Set("X-Real-IP", "198.51.100.33")
		return h.VerifyEmail(c)
	})

	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/verify-email", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		respBody, _ := readBody(resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	pointStore := pointstore.NewPointStore(db)
	points, err := pointStore.GetOrCreateUserPoints(ctx, inviter.ID)
	if err != nil {
		t.Fatalf("get inviter points: %v", err)
	}
	if points.FreeBalance != 0 {
		t.Fatalf("expected inviter free balance=0 when same IP, got %d", points.FreeBalance)
	}

	txns, total, err := pointStore.ListTransactions(ctx, inviter.ID, 1, 20)
	if err != nil {
		t.Fatalf("list inviter transactions: %v", err)
	}
	if total != 0 || len(txns) != 0 {
		t.Fatalf("expected no invite reward transaction, total=%d len=%d", total, len(txns))
	}

	var auditLog model.AuditLog
	if err := db.WithContext(ctx).Where("action = ?", model.AuditInviteRewardSkipped).First(&auditLog).Error; err != nil {
		t.Fatalf("expected invite_reward_skipped audit log: %v", err)
	}
	if auditLog.ResourceID != invitee.ID {
		t.Fatalf("expected skipped audit resource_id=%q, got %q", invitee.ID, auditLog.ResourceID)
	}
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	b := new(bytes.Buffer)
	_, err := b.ReadFrom(resp.Body)
	return b.Bytes(), err
}

func TestEmailVerifiedMiddleware_Enabled(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)

	user := createTestUser(t, users, "unverified@example.com", false)
	_ = settings.Set(context.Background(), store.SettingEmailVerificationEnabled, "true")

	mw := EmailVerifiedMiddleware(users, settings, cache)
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	}, mw, func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestEmailVerifiedMiddleware_Disabled(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)

	user := createTestUser(t, users, "unverified@example.com", false)
	// email_verification_enabled defaults to false

	mw := EmailVerifiedMiddleware(users, settings, cache)
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	}, mw, func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestEmailVerifiedMiddleware_CachesVerifiedUser(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	_ = settings.Set(context.Background(), store.SettingEmailVerificationEnabled, "true")

	user := createTestUser(t, users, "verified@example.com", true)

	mw := EmailVerifiedMiddleware(users, settings, cache)
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	}, mw, func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp1, err := app.Test(req1)
	if err != nil {
		t.Fatal(err)
	}
	if resp1.StatusCode != fiber.StatusOK {
		t.Fatalf("expected first request 200, got %d", resp1.StatusCode)
	}

	user.EmailVerified = false
	if err := users.Update(context.Background(), user); err != nil {
		t.Fatal(err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("expected second request 200 from cache, got %d", resp2.StatusCode)
	}
}

func TestEmailVerifiedMiddleware_UserLookupError(t *testing.T) {
	db := setupTestDB(t)
	users := store.NewUserStore(db)
	cache := store.NewMemoryCache()
	settings := store.NewSettingStore(db, cache)
	_ = settings.Set(context.Background(), store.SettingEmailVerificationEnabled, "true")

	mw := EmailVerifiedMiddleware(users, settings, cache)
	app := fiber.New()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals("user_id", "missing-user-id")
		return c.Next()
	}, mw, func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}

	respBody, _ := readBody(resp)
	var result map[string]any
	_ = json.Unmarshal(respBody, &result)
	if result["error"] != "email_not_verified" {
		t.Fatalf("expected email_not_verified, got %#v", result["error"])
	}
}
