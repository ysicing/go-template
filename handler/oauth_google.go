package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"

	"github.com/ysicing/go-template/model"
)

// googleEndpoint defines Google OAuth2 endpoints.
var googleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// googleOAuth2Config builds an oauth2.Config for Google from a SocialProvider.
func (h *OAuthHandler) googleOAuth2Config(provider *model.SocialProvider, c fiber.Ctx) *oauth2.Config {
	redirectURL := provider.RedirectURL
	if redirectURL == "" {
		redirectURL = deriveRedirectURL(c, "/api/auth/google/callback")
	}
	return &oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     googleEndpoint,
	}
}

// GoogleLogin redirects to Google OAuth authorization page.
func (h *OAuthHandler) GoogleLogin(c fiber.Ctx) error {
	return h.startOAuthLogin(c, "google", h.googleOAuth2Config)
}

// GoogleCallback handles the Google OAuth callback.
func (h *OAuthHandler) GoogleCallback(c fiber.Ctx) error {
	return h.handleOAuthProviderCallback(c, "google", h.googleOAuth2Config, fetchGoogleProfile)
}

// googleUser represents the Google userinfo API response.
type googleUser struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func fetchGoogleProfile(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*oauthProviderProfile, error) {
	u, err := fetchGoogleUser(ctx, cfg, token)
	if err != nil {
		return nil, err
	}
	return &oauthProviderProfile{
		ProviderID: u.ID,
		Email:      u.Email,
		Username:   "g_" + u.ID,
		AvatarURL:  u.Picture,
	}, nil
}

// fetchGoogleUser calls the Google userinfo API with the given token.
func fetchGoogleUser(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*googleUser, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var u googleUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}
