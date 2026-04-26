package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/validator"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

type updateMeRequest struct {
	Username  *string `json:"username"`
	Email     *string `json:"email"`
	AvatarURL *string `json:"avatar_url"`
}

// UpdateMe handles PUT /api/users/me.
func (h *UserHandler) UpdateMe(c fiber.Ctx) error {
	userID, user, err := h.loadCurrentUser(c)
	if err != nil {
		return finishHandlerError(c, err)
	}

	req, err := parseUpdateMeRequest(c)
	if err != nil {
		return finishHandlerError(c, err)
	}

	emailChanged, err := h.applyProfileUpdate(c, user, req)
	if err != nil {
		return finishHandlerError(c, err)
	}
	if err := h.users.Update(c.Context(), user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update user"})
	}

	h.auditProfileUpdate(c, userID)
	h.handleEmailChanged(c, userID, user, emailChanged)
	return c.JSON(fiber.Map{"user": NewUserResponse(user)})
}

func (h *UserHandler) loadCurrentUser(c fiber.Ctx) (string, *model.User, error) {
	userID, _ := c.Locals("user_id").(string)
	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return "", nil, jsonError(fiber.StatusNotFound, "user not found")
	}
	return userID, user, nil
}

func parseUpdateMeRequest(c fiber.Ctx) (*updateMeRequest, error) {
	var req updateMeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return nil, jsonError(fiber.StatusBadRequest, "invalid request body")
	}
	return &req, nil
}

func (h *UserHandler) applyProfileUpdate(c fiber.Ctx, user *model.User, req *updateMeRequest) (bool, error) {
	if err := applyUsernameUpdate(user, req.Username); err != nil {
		return false, err
	}
	emailChanged, err := h.applyEmailUpdate(c, user, req.Email)
	if err != nil {
		return false, err
	}
	if err := applyAvatarUpdate(user, req.AvatarURL); err != nil {
		return false, err
	}
	return emailChanged, nil
}

func applyUsernameUpdate(user *model.User, username *string) error {
	if username == nil {
		return nil
	}
	value := strings.TrimSpace(*username)
	if len(value) < 3 || len(value) > 32 {
		return jsonError(fiber.StatusBadRequest, "username must be 3-32 characters")
	}
	user.Username = value
	return nil
}

func (h *UserHandler) applyEmailUpdate(c fiber.Ctx, user *model.User, email *string) (bool, error) {
	if email == nil {
		return false, nil
	}

	value := strings.TrimSpace(*email)
	if !isValidEmail(value) {
		return false, jsonError(fiber.StatusBadRequest, "invalid email format")
	}
	if value == user.Email {
		return false, nil
	}
	if err := validateEmailChangeCooldown(user.EmailUpdatedAt); err != nil {
		return false, err
	}
	if existing, _ := h.users.GetByEmail(c.Context(), value); existing != nil && existing.ID != user.ID {
		return false, jsonError(fiber.StatusConflict, "email already in use")
	}
	if err := h.validateUpdatedEmailDomain(value); err != nil {
		return false, err
	}

	now := time.Now()
	user.Email = value
	user.EmailVerified = false
	user.EmailUpdatedAt = &now
	return true, nil
}

func validateEmailChangeCooldown(updatedAt *time.Time) error {
	if updatedAt == nil {
		return nil
	}
	daysSince := time.Since(*updatedAt).Hours() / 24
	if daysSince >= 30 {
		return nil
	}
	remaining := int(30 - daysSince)
	return jsonError(fiber.StatusTooManyRequests, fmt.Sprintf("email can only be changed once every 30 days, %d days remaining", remaining))
}

func (h *UserHandler) validateUpdatedEmailDomain(email string) error {
	if h.settings == nil {
		return nil
	}
	mode := h.settings.Get(store.SettingEmailDomainMode, "disabled")
	if mode == "disabled" {
		return nil
	}

	whitelist := h.settings.GetStringSlice(store.SettingEmailDomainWhitelist, nil)
	blacklist := h.settings.GetStringSlice(store.SettingEmailDomainBlacklist, nil)
	if err := validator.ValidateEmailDomain(email, mode, whitelist, blacklist); err != nil {
		return jsonError(fiber.StatusBadRequest, "email domain not allowed")
	}
	return nil
}

func applyAvatarUpdate(user *model.User, avatarURL *string) error {
	if avatarURL == nil {
		return nil
	}
	value := strings.TrimSpace(*avatarURL)
	if value != "" {
		if len(value) > 512 {
			return jsonError(fiber.StatusBadRequest, "avatar_url must be at most 512 characters")
		}
		if !strings.HasPrefix(value, "https://") {
			return jsonError(fiber.StatusBadRequest, "avatar_url must be an HTTPS URL")
		}
	}
	user.AvatarURL = value
	return nil
}

func (h *UserHandler) auditProfileUpdate(c fiber.Ctx, userID string) {
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditUserUpdate,
		Resource:   "user",
		ResourceID: userID,
		Status:     "success",
		Detail:     "user profile updated",
	})
}

func (h *UserHandler) handleEmailChanged(c fiber.Ctx, userID string, user *model.User, emailChanged bool) {
	if !emailChanged || h.emailHandler == nil || h.settings == nil || !h.settings.GetBool(store.SettingEmailVerificationEnabled, false) {
		return
	}

	baseURL := c.Protocol() + "://" + c.Hostname()
	_ = h.emailHandler.SendVerificationEmail(c, user, baseURL)
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditEmailChange,
		Resource:   "user",
		ResourceID: userID,
		Status:     "success",
		Detail:     "user email changed",
		Metadata: map[string]string{
			"email": user.Email,
		},
	})
}
