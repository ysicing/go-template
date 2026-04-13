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

	oauth2github "golang.org/x/oauth2/github"
)

// githubOAuth2Config builds an oauth2.Config for GitHub from a SocialProvider.
func (h *OAuthHandler) githubOAuth2Config(provider *model.SocialProvider, c fiber.Ctx) *oauth2.Config {
	redirectURL := provider.RedirectURL
	if redirectURL == "" {
		redirectURL = deriveRedirectURL(c, "/api/auth/github/callback")
	}
	return &oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"user:email"},
		Endpoint:     oauth2github.Endpoint,
	}
}

// GitHubLogin redirects to GitHub OAuth authorization page.
func (h *OAuthHandler) GitHubLogin(c fiber.Ctx) error {
	return h.startOAuthLogin(c, "github", h.githubOAuth2Config)
}

// GitHubCallback handles the GitHub OAuth callback.
func (h *OAuthHandler) GitHubCallback(c fiber.Ctx) error {
	return h.handleOAuthProviderCallback(c, "github", h.githubOAuth2Config, fetchGitHubProfile)
}

// githubUser represents the GitHub user API response.
type githubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func fetchGitHubProfile(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*oauthProviderProfile, error) {
	u, err := fetchGitHubUser(ctx, cfg, token)
	if err != nil {
		return nil, err
	}
	return &oauthProviderProfile{
		ProviderID: fmt.Sprintf("%d", u.ID),
		Email:      u.Email,
		Username:   "gh_" + u.Login,
		AvatarURL:  u.AvatarURL,
	}, nil
}

// fetchGitHubUser calls the GitHub user API with the given token.
func fetchGitHubUser(ctx context.Context, cfg *oauth2.Config, token *oauth2.Token) (*githubUser, error) {
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}

	if u.Email == "" {
		u.Email = fetchGitHubPrimaryEmail(client)
	}

	return &u, nil
}

// fetchGitHubPrimaryEmail calls /user/emails to get the primary verified email.
func fetchGitHubPrimaryEmail(client *http.Client) string {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return ""
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email
		}
	}
	return ""
}
