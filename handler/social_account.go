package handler

import (
	"context"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

type socialAccountNotifier interface {
	NotifySocialAccountUnlinked(ctx context.Context, provider, providerID string)
	NotifySocialAccountLinked(ctx context.Context, provider, providerID string)
}

// SocialAccountHandler handles user social account management.
type SocialAccountHandler struct {
	socialAccounts *store.SocialAccountStore
	users          *store.UserStore
	audit          *store.AuditLogStore
	notifier       socialAccountNotifier
}

// NewSocialAccountHandler creates a new social account handler.
func NewSocialAccountHandler(socialAccounts *store.SocialAccountStore, users *store.UserStore, audit *store.AuditLogStore, notifier socialAccountNotifier) *SocialAccountHandler {
	return &SocialAccountHandler{
		socialAccounts: socialAccounts,
		users:          users,
		audit:          audit,
		notifier:       notifier,
	}
}

// ListMySocialAccounts handles GET /api/users/me/social-accounts.
func (h *SocialAccountHandler) ListMySocialAccounts(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	accounts, err := h.socialAccounts.ListByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list social accounts"})
	}

	// Format response
	type socialAccountResp struct {
		ID          string `json:"id"`
		Provider    string `json:"provider"`
		ProviderID  string `json:"provider_id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		AvatarURL   string `json:"avatar_url"`
		CreatedAt   string `json:"created_at"`
	}

	result := make([]socialAccountResp, len(accounts))
	for i, acc := range accounts {
		result[i] = socialAccountResp{
			ID:          acc.ID,
			Provider:    acc.Provider,
			ProviderID:  acc.ProviderID,
			Username:    acc.Username,
			DisplayName: acc.DisplayName,
			Email:       acc.Email,
			AvatarURL:   acc.AvatarURL,
			CreatedAt:   acc.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return c.JSON(result)
}

// UnlinkSocialAccount handles DELETE /api/users/me/social-accounts/:id.
func (h *SocialAccountHandler) UnlinkSocialAccount(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	accountID := c.Params("id")
	if accountID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "account id required"})
	}

	// Verify the account belongs to the user
	accounts, err := h.socialAccounts.ListByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to verify account"})
	}

	var foundAccount *model.SocialAccount
	for _, acc := range accounts {
		if acc.ID == accountID {
			foundAccount = acc
			break
		}
	}

	if foundAccount == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "social account not found"})
	}

	// Check if this is the last social account and user has no password
	// (prevent user from being locked out)
	if len(accounts) == 1 {
		user, err := h.users.GetByID(c.Context(), userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to verify user"})
		}
		if user.PasswordHash == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot unlink the last social account without a password set"})
		}
	}

	if err := h.socialAccounts.Delete(c.Context(), accountID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to unlink account"})
	}

	if h.notifier != nil {
		h.notifier.NotifySocialAccountUnlinked(c.Context(), foundAccount.Provider, foundAccount.ProviderID)
	}

	// Audit log
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditSocialAccountUnlink,
		Resource:   "social_account",
		ResourceID: accountID,
		Status:     "success",
		Detail:     "social account unlinked",
		Metadata: map[string]string{
			"provider":    foundAccount.Provider,
			"provider_id": foundAccount.ProviderID,
			"username":    foundAccount.Username,
		},
	})

	return c.JSON(fiber.Map{"message": "social account unlinked"})
}
