package handler

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ysicing/go-template/model"

	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
)

type socialLinkPendingData struct {
	UserID     string `json:"user_id"`
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
	Email      string `json:"email"`
	AvatarURL  string `json:"avatar_url"`
}

func socialLinkPendingKey(token string) string {
	return "social_link_pending:" + token
}

func socialLinkWebAuthnKey(token string) string {
	return "social_link_webauthn:" + token
}

func (h *OAuthHandler) loadPendingSocialLink(ctx context.Context, linkToken string) (*socialLinkPendingData, error) {
	val, err := h.cache.Get(ctx, socialLinkPendingKey(linkToken))
	if err != nil || val == "" {
		return nil, errors.New("invalid_or_expired_link_token")
	}

	var pending socialLinkPendingData
	if err := json.Unmarshal([]byte(val), &pending); err != nil {
		return nil, errors.New("invalid_link_data")
	}
	return &pending, nil
}

func (h *OAuthHandler) completeSocialLink(c fiber.Ctx, user *model.User, pending *socialLinkPendingData, verificationMethod string, cleanupKeys ...string) error {
	newAccount := &model.SocialAccount{
		UserID:     user.ID,
		Provider:   pending.Provider,
		ProviderID: pending.ProviderID,
		Email:      pending.Email,
		AvatarURL:  pending.AvatarURL,
	}
	if err := h.socialAccounts.Create(c.Context(), newAccount); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to link social account"})
	}

	updated := false
	if user.AvatarURL == "" && pending.AvatarURL != "" {
		user.AvatarURL = pending.AvatarURL
		updated = true
	}
	if !user.EmailVerified {
		user.EmailVerified = true
		updated = true
	}
	if updated {
		_ = h.users.Update(c.Context(), user)
	}

	for _, key := range cleanupKeys {
		if key == "" {
			continue
		}
		_ = h.cache.Del(c.Context(), key)
	}

	clearFailedAuthAttempts(c.Context(), h.cache, user.ID)

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     user.ID,
		Action:     model.AuditSocialAccountLink,
		Resource:   "social_account",
		ResourceID: newAccount.ID,
		Status:     "success",
		Detail:     "social account linked after verification",
		Metadata: map[string]string{
			"provider":            pending.Provider,
			"provider_id":         pending.ProviderID,
			"verification_method": verificationMethod,
		},
	})

	return h.respondWithTokens(c, user, pending.Provider)
}

// ExchangeCode handles POST /api/auth/social/exchange — exchanges a temporary code for tokens.
func (h *OAuthHandler) ExchangeCode(c fiber.Ctx) error {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code is required"})
	}

	val, err := h.cache.Get(c.Context(), "oauth_code:"+req.Code)
	if err != nil || val == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired code"})
	}
	_ = h.cache.Del(c.Context(), "oauth_code:"+req.Code)

	var codeData struct {
		UserID   string `json:"user_id"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal([]byte(val), &codeData); err != nil || codeData.UserID == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "invalid code data"})
	}
	userID, provider := codeData.UserID, codeData.Provider

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	return h.respondWithTokens(c, user, provider)
}

// ConfirmSocialLink handles POST /api/auth/social/confirm-link — confirms linking a social account.
// Supports password or TOTP verification.
func (h *OAuthHandler) ConfirmSocialLink(c fiber.Ctx) error {
	var req struct {
		LinkToken string `json:"link_token"`
		Password  string `json:"password,omitempty"`
		TOTPCode  string `json:"totp_code,omitempty"`
		Challenge string `json:"challenge,omitempty"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if req.LinkToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "link_token is required"})
	}
	if req.Challenge != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "challenge verification is not supported"})
	}
	if req.Password == "" && req.TOTPCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "verification required: password or totp_code"})
	}

	pending, err := h.loadPendingSocialLink(c.Context(), req.LinkToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired link token"})
	}

	user, err := h.users.GetByID(c.Context(), pending.UserID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	if isAccountLocked(c.Context(), h.cache, user.ID) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "account temporarily locked, try again later"})
	}

	verified := false
	verificationMethod := ""

	if req.Password != "" {
		if user.PasswordHash == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "password_not_set",
				"hint":  "This account was created via social login. Please use TOTP or set a password first.",
			})
		}
		if user.CheckPassword(req.Password) {
			verified = true
			verificationMethod = "password"
		} else {
			_ = recordAuditFromFiber(c, h.audit, AuditEvent{
				UserID:   user.ID,
				Action:   model.AuditSocialAccountLink,
				Resource: "social_account",
				Status:   "failed",
				Detail:   "social link verification failed",
				Metadata: map[string]string{
					"reason": "invalid_password",
				},
			})
			recordFailedAuthAttempt(c.Context(), h.cache, user.ID)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid password"})
		}
	}

	if !verified && req.TOTPCode != "" {
		if h.mfa == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "MFA not available"})
		}
		mfaCfg, err := h.mfa.GetByUserID(c.Context(), user.ID)
		if err != nil || !mfaCfg.TOTPEnabled {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "totp_not_enabled",
				"hint":  "TOTP is not enabled for this account. Please use password or set up TOTP first.",
			})
		}
		if totp.Validate(req.TOTPCode, mfaCfg.TOTPSecret) {
			verified = true
			verificationMethod = "totp"
		} else {
			_ = recordAuditFromFiber(c, h.audit, AuditEvent{
				UserID:   user.ID,
				Action:   model.AuditSocialAccountLink,
				Resource: "social_account",
				Status:   "failed",
				Detail:   "social link verification failed",
				Metadata: map[string]string{
					"reason": "invalid_totp",
				},
			})
			recordFailedAuthAttempt(c.Context(), h.cache, user.ID)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid TOTP code"})
		}
	}

	if !verified {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "verification failed"})
	}

	return h.completeSocialLink(c, user, pending, verificationMethod, socialLinkPendingKey(req.LinkToken))
}
