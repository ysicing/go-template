package httpserver_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/buildinfo"
	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
	"github.com/ysicing/go-template/internal/httpserver"
	"github.com/ysicing/go-template/internal/setup"
	"github.com/ysicing/go-template/internal/user"
	"gorm.io/gorm"
)

type apiResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type testAppEnv struct {
	app         *fiber.App
	conn        *gorm.DB
	authService *auth.Service
	tokens      *auth.TokenManager
}

func TestSetupStatusRoute(t *testing.T) {
	app := httpserver.NewForTest(true)
	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthMeRouteRequiresAuth(t *testing.T) {
	app := httpserver.NewForTest(false)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAdminUsersRouteRequiresAuth(t *testing.T) {
	app := httpserver.NewForTest(false)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAdminUsersRouteRejectsNonAdmin(t *testing.T) {
	env := newTestAppEnv(t)
	member := env.createUser(t, "member", "member@example.com", "password123", user.RoleUser)

	resp := env.request(t, http.MethodGet, "/api/admin/users", "", member)
	body := readAPIResponse(t, resp)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	if body.Code != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN, got %s", body.Code)
	}
}

func TestAdminUserGetRouteReturnsUserForAdmin(t *testing.T) {
	env := newTestAppEnv(t)
	admin := env.createUser(t, "admin", "admin@example.com", "password123", user.RoleAdmin)
	member := env.createUser(t, "member", "member@example.com", "password123", user.RoleUser)

	resp := env.request(t, http.MethodGet, "/api/admin/users/"+itoa(member.ID), "", admin)
	body := readAPIResponse(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body.Code != "OK" {
		t.Fatalf("expected OK, got %s", body.Code)
	}

	var data struct {
		User user.User `json:"user"`
	}
	decodeJSON(t, body.Data, &data)
	if data.User.ID != member.ID {
		t.Fatalf("expected user id %d, got %d", member.ID, data.User.ID)
	}
	if data.User.Username != member.Username {
		t.Fatalf("expected username %s, got %s", member.Username, data.User.Username)
	}
}

func TestChangePasswordRouteRequiresAuth(t *testing.T) {
	app := httpserver.NewForTest(false)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLoginRouteRejectsInvalidCredentialsWithStableError(t *testing.T) {
	env := newTestAppEnv(t)
	env.createUser(t, "member", "member@example.com", "password123", user.RoleUser)

	resp := env.request(t, http.MethodPost, "/api/auth/login", `{"identifier":"member","password":"wrong-password"}`, nil)
	body := readAPIResponse(t, resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if body.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("expected INVALID_CREDENTIALS, got %s", body.Code)
	}
	if body.Message != "invalid credentials" {
		t.Fatalf("expected invalid credentials message, got %s", body.Message)
	}
}

func TestRefreshRouteRejectsInvalidTokenWithStableError(t *testing.T) {
	env := newTestAppEnv(t)

	resp := env.request(t, http.MethodPost, "/api/auth/refresh", `{"refresh_token":"not-a-jwt"}`, nil)
	body := readAPIResponse(t, resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if body.Code != "INVALID_REFRESH_TOKEN" {
		t.Fatalf("expected INVALID_REFRESH_TOKEN, got %s", body.Code)
	}
	if body.Message != "invalid refresh token" {
		t.Fatalf("expected invalid refresh token message, got %s", body.Message)
	}
}

func TestLoginRouteReturnsStableInternalErrorWhenDatabaseFails(t *testing.T) {
	env := newTestAppEnv(t)
	env.closeDB(t)

	resp := env.request(t, http.MethodPost, "/api/auth/login", `{"identifier":"member","password":"password123"}`, nil)
	body := readAPIResponse(t, resp)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
	if body.Code != "LOGIN_FAILED" {
		t.Fatalf("expected LOGIN_FAILED, got %s", body.Code)
	}
	if body.Message != "failed to login" {
		t.Fatalf("expected stable login failure message, got %s", body.Message)
	}
}

func TestAuthMeRouteReturnsStableInternalErrorWhenDatabaseFails(t *testing.T) {
	env := newTestAppEnv(t)
	member := env.createUser(t, "member", "member@example.com", "password123", user.RoleUser)
	env.closeDB(t)

	resp := env.request(t, http.MethodGet, "/api/auth/me", "", member)
	body := readAPIResponse(t, resp)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
	if body.Code != "AUTH_ME_FAILED" {
		t.Fatalf("expected AUTH_ME_FAILED, got %s", body.Code)
	}
	if body.Message != "failed to load current user" {
		t.Fatalf("expected stable current user failure message, got %s", body.Message)
	}
}

func TestChangePasswordRouteChangesPasswordForAuthenticatedUser(t *testing.T) {
	env := newTestAppEnv(t)
	member := env.createUser(t, "member", "member@example.com", "oldpass123", user.RoleUser)

	resp := env.request(t, http.MethodPost, "/api/auth/change-password", `{"old_password":"oldpass123","new_password":"newpass123","confirm_new_password":"newpass123"}`, member)
	body := readAPIResponse(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body.Code != "OK" {
		t.Fatalf("expected OK, got %s", body.Code)
	}

	var data struct {
		Changed bool `json:"changed"`
	}
	decodeJSON(t, body.Data, &data)
	if !data.Changed {
		t.Fatal("expected changed=true")
	}

	if _, _, err := env.authService.Login(member.Username, "oldpass123"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected old password to fail, got %v", err)
	}
	if _, _, err := env.authService.Login(member.Username, "newpass123"); err != nil {
		t.Fatalf("expected new password login to work, got %v", err)
	}
}

func TestAdminCreateUserRouteReturnsConflictForDuplicateUsername(t *testing.T) {
	env := newTestAppEnv(t)
	admin := env.createUser(t, "admin", "admin@example.com", "password123", user.RoleAdmin)
	env.createUser(t, "member", "member@example.com", "password123", user.RoleUser)

	resp := env.request(t, http.MethodPost, "/api/admin/users", `{"username":"member","email":"other@example.com","password":"password123","role":"user","status":"active"}`, admin)
	body := readAPIResponse(t, resp)

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	if body.Code != "DUPLICATE_USERNAME" {
		t.Fatalf("expected DUPLICATE_USERNAME, got %s", body.Code)
	}
}

func TestHealthzRoute(t *testing.T) {
	app := httpserver.NewForTest(true)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSwaggerUIRoute(t *testing.T) {
	app := httpserver.NewForTest(true)
	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestOpenAPIDocRoute(t *testing.T) {
	app := httpserver.NewForTest(true)
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var doc struct {
		Info struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode openapi doc: %v", err)
	}
	if doc.Info.Title != "go-template API" {
		t.Fatalf("expected openapi title, got %q", doc.Info.Title)
	}
	if doc.Info.Version != buildinfo.FullVersion() {
		t.Fatalf("expected openapi version %q, got %q", buildinfo.FullVersion(), doc.Info.Version)
	}
	if _, ok := doc.Paths["/api/auth/login"]; !ok {
		t.Fatalf("expected /api/auth/login in swagger paths")
	}
}

func TestSystemVersionRouteReturnsBuildMetadata(t *testing.T) {
	app := httpserver.NewForTest(false)
	req := httptest.NewRequest(http.MethodGet, "/api/system/version", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := readAPIResponse(t, resp)
	if body.Code != "OK" {
		t.Fatalf("expected OK, got %s", body.Code)
	}

	var data struct {
		FullVersion string `json:"full_version"`
		Version     string `json:"version"`
		Commit      string `json:"commit"`
		BuildTime   string `json:"build_time"`
	}
	decodeJSON(t, body.Data, &data)
	if data.FullVersion == "" {
		t.Fatal("expected full_version to be present")
	}
	if data.Version == "" {
		t.Fatal("expected version to be present")
	}
	if data.Commit == "" {
		t.Fatal("expected commit to be present")
	}
	if data.BuildTime == "" {
		t.Fatal("expected build_time to be present")
	}
}

func newTestAppEnv(t *testing.T) *testAppEnv {
	t.Helper()

	dsn := "file:" + filepath.Join(t.TempDir(), "httpserver.db") + "?_pragma=foreign_keys(1)"
	conn, err := db.Open(config.DatabaseConfig{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(conn); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	tokens := auth.NewTokenManager("issuer", "secret", config.Duration("15m").Value(), config.Duration("1h").Value())
	userService := user.NewService(conn)
	authService := auth.NewService(conn, tokens)

	app := httpserver.New(httpserver.Dependencies{
		DB:            conn,
		UserService:   userService,
		Auth:          authService,
		Tokens:        tokens,
		Setup:         setup.NewService("configs/config.yaml", conn),
		SetupRequired: false,
	})

	return &testAppEnv{app: app, conn: conn, authService: authService, tokens: tokens}
}

func (env *testAppEnv) createUser(t *testing.T, username string, email string, password string, role user.Role) *user.User {
	t.Helper()

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	account := &user.User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		Status:       "active",
	}
	if err := env.conn.Create(account).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return account
}

func (env *testAppEnv) request(t *testing.T, method string, path string, body string, account *user.User) *http.Response {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if account != nil {
		pair, err := env.tokens.Issue(account.ID, string(account.Role))
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	}

	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	return resp
}

func (env *testAppEnv) closeDB(t *testing.T) {
	t.Helper()

	sqlDB, err := env.conn.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
}

func readAPIResponse(t *testing.T, resp *http.Response) apiResponse {
	t.Helper()
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var body apiResponse
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return body
}

func decodeJSON(t *testing.T, input []byte, out any) {
	t.Helper()

	if err := json.Unmarshal(input, out); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
