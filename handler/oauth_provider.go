package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"
)

type oauthProviderProfile struct {
	ProviderID string
	Email      string
	Username   string
	AvatarURL  string
}

type oauthConfigBuilder func(*model.SocialProvider, fiber.Ctx) *oauth2.Config

type oauthProfileFetcher func(context.Context, *oauth2.Config, *oauth2.Token) (*oauthProviderProfile, error)

// errProviderHandled is a sentinel indicating the response was already sent.
var errProviderHandled = errors.New("provider response handled")

// saveOAuthState stores state in cache.
// linkUserID is optional; when set it is stored server-side so the state string itself
// does not embed sensitive user information.
func (h *OAuthHandler) saveOAuthState(c fiber.Ctx, state, linkUserID string) {
	val := "1"
	if linkUserID != "" {
		val = "link:" + linkUserID
	}
	_ = h.cache.Set(c.Context(), oauthStateKey(state), val, 5*time.Minute)
}

// verifyOAuthState validates state and returns (valid, linkUserID).
// Cookie fallback is intentionally removed to prevent CSRF attacks.
// SetNX ensures only one callback can consume a given state value (replay protection).
func (h *OAuthHandler) verifyOAuthState(c fiber.Ctx, state string) (bool, string) {
	if state == "" {
		return false, ""
	}
	val, err := h.cache.Get(c.Context(), oauthStateKey(state))
	if err != nil || val == "" {
		return false, ""
	}
	consumed, err := h.cache.SetNX(c.Context(), oauthStateUsedKey(state), "1", 5*time.Minute)
	if err != nil || !consumed {
		return false, ""
	}
	_ = h.cache.Del(c.Context(), oauthStateKey(state))

	linkUserID := ""
	if strings.HasPrefix(val, "link:") {
		linkUserID = strings.TrimPrefix(val, "link:")
	}
	return true, linkUserID
}

func oauthStateKey(state string) string     { return "oauth_state:" + state }
func oauthStateUsedKey(state string) string { return "oauth_state_used:" + state }

// generateOAuthState creates a cryptographically random state string for CSRF protection.
// The userID (for link operations) is stored server-side via saveOAuthState, not in the state value.
func generateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// deriveRedirectURL builds a redirect URL from the request's scheme and host.
func deriveRedirectURL(c fiber.Ctx, path string) string {
	scheme := c.Scheme()
	host := c.Hostname()
	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

// loadProvider loads a social provider by name and checks it is enabled.
// On failure it writes the HTTP response and returns errProviderHandled.
func (h *OAuthHandler) loadProvider(c fiber.Ctx, name string) (*model.SocialProvider, error) {
	provider, err := h.providers.GetByName(c.Context(), name)
	if err != nil {
		if errors.Is(err, store.ErrSocialProviderSecretUnavailable) {
			_ = c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": name + " oauth temporarily unavailable"})
			return nil, errProviderHandled
		}
		_ = c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": name + " oauth not configured"})
		return nil, errProviderHandled
	}
	if !provider.Enabled {
		_ = c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": name + " oauth not configured"})
		return nil, errProviderHandled
	}
	return provider, nil
}

func (h *OAuthHandler) startOAuthLogin(c fiber.Ctx, name string, configBuilder oauthConfigBuilder) error {
	provider, err := h.loadProvider(c, name)
	if err != nil {
		return nil
	}
	cfg := configBuilder(provider, c)

	userID := ""
	if uid, ok := c.Locals("user_id").(string); ok {
		userID = uid
	}

	state, err := generateOAuthState()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate state"})
	}
	h.saveOAuthState(c, state, userID)
	return c.Redirect().To(cfg.AuthCodeURL(state))
}

func (h *OAuthHandler) handleOAuthProviderCallback(c fiber.Ctx, name string, configBuilder oauthConfigBuilder, fetchProfile oauthProfileFetcher) error {
	provider, err := h.loadProvider(c, name)
	if err != nil {
		return nil
	}

	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing code"})
	}

	state := c.Query("state")
	valid, linkUserID := h.verifyOAuthState(c, state)
	if !valid {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid oauth state"})
	}
	isLink := linkUserID != ""

	cfg := configBuilder(provider, c)
	token, tokenErr := cfg.Exchange(c.Context(), code)
	if tokenErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "failed to exchange token"})
	}

	profile, fetchErr := fetchProfile(c.Context(), cfg, token)
	if fetchErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get user info"})
	}

	if isLink && linkUserID != "" {
		return h.handleSocialLink(c, linkUserID, name, profile.ProviderID, profile.Email, profile.AvatarURL)
	}

	user, lookupErr := h.users.GetByProviderID(c.Context(), name, profile.ProviderID)
	if lookupErr != nil {
		user, err = h.linkOrCreateSocialUser(c.Context(), name, profile.ProviderID, profile.Email, profile.Username, profile.AvatarURL)
		if err != nil {
			errMsg := err.Error()
			if errMsg == "email_required" {
				return c.Redirect().To("/login?error=email_required")
			}
			if errMsg == "registration_disabled" {
				return c.Redirect().To("/login?error=registration_disabled")
			}
			var linkErr *errAccountLinkRequired
			if errors.As(err, &linkErr) {
				return c.Redirect().To("/login?link_required=true&link_token=" + linkErr.LinkToken + "&provider=" + name)
			}
			logger.L.Error().Err(err).Str("provider", name).Msg("social login failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "social login failed"})
		}
	}

	return h.handleSocialCallback(c, user, name)
}
