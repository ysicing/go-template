package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
)

type openAPITestDocument struct {
	Paths map[string]map[string]any `json:"paths"`
}

func TestRegisterDocsRoutes_OpenAPIFiltersByPermissions(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	registerDocsRoutes(app, deps, BuildInfo{Version: "test"})

	unauthDoc := fetchOpenAPIDoc(t, app, "")
	assertHasPath(t, unauthDoc, "/health")
	assertHasPath(t, unauthDoc, "/api/auth/login")
	assertMissingPath(t, unauthDoc, "/api/users/me")
	assertMissingPath(t, unauthDoc, "/api/admin/users")

	user := &model.User{
		Username:     "docs-user",
		Email:        "docs-user@example.com",
		Provider:     "local",
		ProviderID:   "docs-user",
		TokenVersion: 1,
	}
	if err := deps.DB.WithContext(context.Background()).Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	userToken, err := handler.GenerateAccessToken(user.ID, false, nil, user.TokenVersion, deps.Config.JWT.Secret, deps.Config.JWT.Issuer, time.Hour)
	if err != nil {
		t.Fatalf("generate user token: %v", err)
	}
	userDoc := fetchOpenAPIDoc(t, app, userToken)
	assertHasPath(t, userDoc, "/api/users/me")
	assertMissingPath(t, userDoc, "/api/admin/users")

	permToken, err := handler.GenerateAccessToken(user.ID, false, []string{model.PermissionAdminUsersRead}, user.TokenVersion, deps.Config.JWT.Secret, deps.Config.JWT.Issuer, time.Hour)
	if err != nil {
		t.Fatalf("generate permission token: %v", err)
	}
	permDoc := fetchOpenAPIDoc(t, app, permToken)
	assertHasPath(t, permDoc, "/api/admin/users")
	assertMissingPath(t, permDoc, "/api/admin/settings")

	adminToken, err := handler.GenerateAccessToken(user.ID, true, model.AllAdminPermissions(), user.TokenVersion, deps.Config.JWT.Secret, deps.Config.JWT.Issuer, time.Hour)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}
	adminDoc := fetchOpenAPIDoc(t, app, adminToken)
	assertHasPath(t, adminDoc, "/api/admin/settings")
	assertHasPath(t, adminDoc, "/api/admin/users")
}

func TestRegisterDocsRoutes_SwaggerUI(t *testing.T) {
	app := fiber.New()
	deps := testRouteDeps(t)
	registerDocsRoutes(app, deps, BuildInfo{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET /swagger/index.html: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected swagger ui status 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(strings.ToLower(string(body)), "swagger") {
		t.Fatalf("expected swagger html response, got %s", string(body))
	}
}

func fetchOpenAPIDoc(t *testing.T, app *fiber.App, token string) openAPITestDocument {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET /openapi.json: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}
	var doc openAPITestDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatalf("decode openapi json: %v", err)
	}
	return doc
}

func assertHasPath(t *testing.T, doc openAPITestDocument, path string) {
	t.Helper()
	if _, ok := doc.Paths[path]; !ok {
		t.Fatalf("expected path %s to exist, got %#v", path, doc.Paths)
	}
}

func assertMissingPath(t *testing.T, doc openAPITestDocument, path string) {
	t.Helper()
	if _, ok := doc.Paths[path]; ok {
		t.Fatalf("expected path %s to be hidden, got %#v", path, doc.Paths[path])
	}
}
