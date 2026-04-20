package handler

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

// GitHubCompatHandler provides GitHub-compatible OAuth API endpoints so that
// applications supporting only GitHub OAuth can use this service as a drop-in
// replacement by pointing their endpoint URLs here.
type GitHubCompatHandler struct {
	oidcHandler http.Handler
	oidcStorage *store.OIDCStorage
}

func NewGitHubCompatHandler(oidcHandler http.Handler, oidcStorage *store.OIDCStorage) *GitHubCompatHandler {
	return &GitHubCompatHandler{oidcHandler: oidcHandler, oidcStorage: oidcStorage}
}

// Authorize redirects to the OIDC authorize endpoint with proper parameters.
func (h *GitHubCompatHandler) Authorize(c fiber.Ctx) error {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	scope := c.Query("scope")
	state := c.Query("state")

	oidcScope := mapGitHubScopes(scope)

	u := fmt.Sprintf("/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(oidcScope),
		url.QueryEscape(state),
	)
	return c.Redirect().To(u)
}

// AccessToken exchanges an authorization code for a token, returning a
// GitHub-compatible response.
func (h *GitHubCompatHandler) AccessToken(c fiber.Ctx) error {
	clientID := c.FormValue("client_id")
	clientSecret := c.FormValue("client_secret")
	code := c.FormValue("code")
	redirectURI := c.FormValue("redirect_uri")

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}

	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = c.Hostname()
	if fh := c.Get("X-Forwarded-Host"); fh != "" {
		req.Header.Set("X-Forwarded-Host", fh)
	}
	if fp := c.Get("X-Forwarded-Proto"); fp != "" {
		req.Header.Set("X-Forwarded-Proto", fp)
	}

	rec := httptest.NewRecorder()
	h.oidcHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		c.Set("Content-Type", rec.Header().Get("Content-Type"))
		return c.Status(rec.Code).Send(rec.Body.Bytes())
	}

	var oidcResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &oidcResp); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "invalid token response"})
	}

	ghResp := fiber.Map{
		"access_token": oidcResp["access_token"],
		"token_type":   "bearer",
		"scope":        "user:email",
	}

	accept := c.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return c.JSON(ghResp)
	}
	// GitHub default: application/x-www-form-urlencoded
	c.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.SendString(fmt.Sprintf("access_token=%s&token_type=bearer&scope=user%%3Aemail",
		url.QueryEscape(fmt.Sprint(oidcResp["access_token"]))))
}

// User returns user info in GitHub-compatible JSON format.
func (h *GitHubCompatHandler) User(c fiber.Ctx) error {
	token := extractOAuthBearerToken(c)
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Bad credentials"})
	}

	user, err := h.oidcStorage.FindUserByAccessToken(c.Context(), token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Bad credentials"})
	}

	// Use SHA-256 hash of user ID (first 8 bytes as uint64) for stable numeric ID
	hash := sha256.Sum256([]byte(user.ID))
	numericID := binary.BigEndian.Uint64(hash[:8])

	return c.JSON(fiber.Map{
		"id":         numericID,
		"login":      user.Username,
		"name":       user.Username,
		"email":      user.Email,
		"avatar_url": user.AvatarURL,
		"node_id":    user.ID,
	})
}

// UserEmails returns user emails in GitHub-compatible JSON format.
func (h *GitHubCompatHandler) UserEmails(c fiber.Ctx) error {
	token := extractOAuthBearerToken(c)
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Bad credentials"})
	}

	user, err := h.oidcStorage.FindUserByAccessToken(c.Context(), token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Bad credentials"})
	}

	return c.JSON([]fiber.Map{{
		"email":      user.Email,
		"primary":    true,
		"verified":   user.EmailVerified,
		"visibility": "public",
	}})
}

// extractOAuthBearerToken extracts the token from Authorization header.
// Supports "Bearer xxx", "token xxx" (GitHub style), and "bearer xxx".
func extractOAuthBearerToken(c fiber.Ctx) string {
	auth := c.Get("Authorization")
	if auth == "" {
		return ""
	}
	if idx := strings.Index(auth, " "); idx > 0 {
		prefix := strings.ToLower(auth[:idx])
		if prefix == "bearer" || prefix == "token" {
			return strings.TrimSpace(auth[idx+1:])
		}
	}
	return ""
}

// mapGitHubScopes converts GitHub OAuth scopes to OIDC scopes.
func mapGitHubScopes(scope string) string {
	if scope == "" {
		return "openid profile email"
	}
	parts := strings.FieldsFunc(scope, func(r rune) bool { return r == ',' || r == ' ' })
	hasProfile, hasEmail := false, false
	for _, s := range parts {
		switch strings.TrimSpace(s) {
		case "user", "read:user":
			hasProfile = true
			hasEmail = true
		case "user:email":
			hasEmail = true
		}
	}
	result := "openid"
	if hasProfile {
		result += " profile"
	}
	if hasEmail {
		result += " email"
	}
	if !hasProfile && !hasEmail {
		result = "openid profile email"
	}
	return result
}
