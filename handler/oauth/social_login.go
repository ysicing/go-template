package oauthhandler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

// errAccountLinkRequired is returned when a social login email matches an existing account.
// The LinkToken holds a short-lived cache key with the pending link data.
type errAccountLinkRequired struct {
	LinkToken string
}

func (e *errAccountLinkRequired) Error() string { return "account link required" }

// linkOrCreateSocialUser handles user lookup, linking, or creation for social login.
func (h *OAuthHandler) linkOrCreateSocialUser(ctx context.Context, provider, providerID, email, username, avatarURL string) (*model.User, error) {
	user, found, err := h.findSocialLoginUser(ctx, provider, providerID, avatarURL)
	if found || err != nil {
		return user, err
	}
	if email == "" {
		return nil, fmt.Errorf("email_required")
	}
	if err := h.requireSocialRegistrationEnabled(); err != nil {
		return nil, err
	}
	if err := h.maybeRequireAccountLink(ctx, provider, providerID, email, avatarURL); err != nil {
		return nil, err
	}
	return h.createSocialLoginUser(ctx, provider, providerID, email, username, avatarURL)
}

func (h *OAuthHandler) findSocialLoginUser(ctx context.Context, provider, providerID, avatarURL string) (*model.User, bool, error) {
	socialAccount, err := h.socialAccounts.GetByProviderAndID(ctx, provider, providerID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to get social account: %w", err)
	}

	user, err := h.users.GetByID(ctx, socialAccount.UserID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get user")
	}
	if avatarURL != "" && socialAccount.AvatarURL != avatarURL {
		socialAccount.AvatarURL = avatarURL
		_ = h.socialAccounts.Update(ctx, socialAccount)
	}
	return user, true, nil
}

func (h *OAuthHandler) requireSocialRegistrationEnabled() error {
	if h.settings.GetBool(store.SettingRegisterEnabled, true) {
		return nil
	}
	return fmt.Errorf("registration_disabled")
}

func (h *OAuthHandler) maybeRequireAccountLink(ctx context.Context, provider, providerID, email, avatarURL string) error {
	existingUser, err := h.users.GetByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get user by email: %w", err)
	}

	linkToken, err := generateSocialLinkToken()
	if err != nil {
		return err
	}
	if err := h.storePendingSocialLink(ctx, linkToken, existingUser.ID, provider, providerID, email, avatarURL); err != nil {
		return err
	}
	return &errAccountLinkRequired{LinkToken: linkToken}
}

func generateSocialLinkToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate link token")
	}
	return hex.EncodeToString(tokenBytes), nil
}

func (h *OAuthHandler) storePendingSocialLink(ctx context.Context, linkToken, userID, provider, providerID, email, avatarURL string) error {
	pendingData := map[string]string{
		"user_id":     userID,
		"provider":    provider,
		"provider_id": providerID,
		"email":       email,
		"avatar_url":  avatarURL,
	}
	dataJSON, _ := json.Marshal(pendingData)
	if err := h.cache.Set(ctx, socialLinkPendingKey(linkToken), string(dataJSON), 5*time.Minute); err != nil {
		return fmt.Errorf("failed to store pending link")
	}
	return nil
}

func (h *OAuthHandler) createSocialLoginUser(ctx context.Context, provider, providerID, email, username, avatarURL string) (*model.User, error) {
	user := &model.User{
		Username:      username,
		Email:         email,
		Provider:      provider,
		ProviderID:    providerID,
		AvatarURL:     avatarURL,
		EmailVerified: true,
	}

	if err := h.socialAccounts.CreateUserWithSocialAccount(ctx, user, &model.SocialAccount{
		Provider:   provider,
		ProviderID: providerID,
		Email:      email,
		AvatarURL:  avatarURL,
	}); err != nil {
		return nil, err
	}
	return user, nil
}

// handleSocialCallback stores user info in a temporary code and redirects to the SPA.
func (h *OAuthHandler) handleSocialCallback(c fiber.Ctx, user *model.User, provider string) error {
	if handlercommon.IsAccountLocked(c.Context(), h.cache, user.ID) {
		return c.Redirect().To("/login?error=account_locked")
	}

	code := store.GenerateRandomToken()
	codeData, _ := json.Marshal(map[string]string{"user_id": user.ID, "provider": provider})
	_ = h.cache.Set(c.Context(), "oauth_code:"+code, string(codeData), 2*time.Minute)
	return c.Redirect().To("/login/callback?code=" + code)
}

// handleSocialLink handles linking a social account to an existing user.
func (h *OAuthHandler) handleSocialLink(c fiber.Ctx, userID, provider, providerID, email, avatarURL string) error {
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Redirect().To("/profile?error=user_not_found")
	}

	existingAccount, err := h.socialAccounts.GetByProviderAndID(c.Context(), provider, providerID)
	if err == nil && existingAccount.UserID != userID {
		return c.Redirect().To("/profile?error=account_already_linked")
	}

	if err != nil {
		newAccount := &model.SocialAccount{
			UserID:     userID,
			Provider:   provider,
			ProviderID: providerID,
			Email:      email,
			AvatarURL:  avatarURL,
		}
		if err := h.socialAccounts.Create(c.Context(), newAccount); err != nil {
			return c.Redirect().To("/profile?error=failed_to_link")
		}
	} else {
		existingAccount.Email = email
		existingAccount.AvatarURL = avatarURL
		_ = h.socialAccounts.Update(c.Context(), existingAccount)
	}

	if user.AvatarURL == "" && avatarURL != "" {
		user.AvatarURL = avatarURL
		_ = h.users.Update(c.Context(), user)
	}

	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
		UserID:   userID,
		Action:   model.AuditSocialAccountLink,
		Resource: "social_account",
		Status:   "success",
		Detail:   "social account linked",
		Metadata: map[string]string{
			"provider": provider,
		},
	})

	return c.Redirect().To("/profile?success=account_linked")
}
