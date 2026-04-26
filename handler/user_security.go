package handler

import (
	"github.com/ysicing/go-template/model"

	"github.com/gofiber/fiber/v3"
)

const passwordHistoryKeep = 5

// ChangePassword handles PUT /api/users/me/password.
func (h *UserHandler) ChangePassword(c fiber.Ctx) error {
	userID, user, err := h.loadCurrentUser(c)
	if err != nil {
		return finishHandlerError(c, err)
	}
	if user.PasswordHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no password set, use set-password first"})
	}

	req, err := parseChangePasswordRequest(c)
	if err != nil {
		return finishHandlerError(c, err)
	}
	if err := h.validatePasswordChange(c, userID, user, req); err != nil {
		return finishHandlerError(c, err)
	}

	if err := h.persistPasswordChange(c, userID, user, req.NewPassword); err != nil {
		return finishHandlerError(c, err)
	}
	h.auditPasswordChange(c, userID)
	return c.JSON(fiber.Map{"message": "password updated, please login again"})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func parseChangePasswordRequest(c fiber.Ctx) (*changePasswordRequest, error) {
	var req changePasswordRequest
	if err := c.Bind().JSON(&req); err != nil {
		return nil, jsonError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return nil, jsonError(fiber.StatusBadRequest, "current_password and new_password are required")
	}
	return &req, nil
}

func (h *UserHandler) validatePasswordChange(c fiber.Ctx, userID string, user *model.User, req *changePasswordRequest) error {
	if !user.CheckPassword(req.CurrentPassword) {
		return jsonError(fiber.StatusUnauthorized, "invalid current password")
	}
	if user.CheckPassword(req.NewPassword) {
		return jsonError(fiber.StatusBadRequest, "new password must be different from current password")
	}
	if shouldEnforcePasswordPolicy(h.settings) {
		if err := model.ValidatePasswordStrength(req.NewPassword); err != nil {
			return jsonError(fiber.StatusBadRequest, err.Error())
		}
	}
	reused, err := h.passwordHistory.IsRecentlyUsed(c.Context(), userID, req.NewPassword, passwordHistoryKeep)
	if err != nil {
		return jsonError(fiber.StatusInternalServerError, "failed to validate password history")
	}
	if reused {
		return jsonError(fiber.StatusBadRequest, "new password was used recently")
	}
	return nil
}

func (h *UserHandler) persistPasswordChange(c fiber.Ctx, userID string, user *model.User, newPassword string) error {
	oldPasswordHash := user.PasswordHash
	if err := user.SetPassword(newPassword); err != nil {
		return jsonError(fiber.StatusInternalServerError, "failed to hash password")
	}
	if user.TokenVersion < 1 {
		user.TokenVersion = 1
	}
	user.TokenVersion++
	if err := h.users.ChangePasswordWithHistory(c.Context(), user, oldPasswordHash); err != nil {
		return jsonError(fiber.StatusInternalServerError, "failed to update password")
	}

	_ = h.passwordHistory.TrimByUserID(c.Context(), userID, passwordHistoryKeep)
	_ = h.refreshTokens.DeleteByUserID(c.Context(), userID)
	if h.cache != nil {
		_ = h.cache.Del(c.Context(), "token_ver:"+userID)
	}
	return nil
}

func (h *UserHandler) auditPasswordChange(c fiber.Ctx, userID string) {
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:     userID,
		Action:     model.AuditPasswordChange,
		Resource:   "user",
		ResourceID: userID,
		Status:     "success",
		Detail:     "password changed",
	})
}

// RevokeAllSessions handles DELETE /api/sessions/.
func (h *UserHandler) RevokeAllSessions(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	if err := h.refreshTokens.DeleteByUserID(c.Context(), userID); err != nil {
		h.auditRevokeAllFailure(c, userID, "refresh_tokens_revoke_failed")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke all sessions"})
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		h.auditRevokeAllFailure(c, userID, "user_not_found")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke all sessions"})
	}
	if err := h.bumpUserTokenVersion(c, userID, user); err != nil {
		return finishHandlerError(c, err)
	}
	if h.cache != nil {
		_ = h.cache.Del(c.Context(), "token_ver:"+userID)
	}

	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:   userID,
		Action:   model.AuditSessionRevoke,
		Resource: "session",
		Status:   "success",
		Detail:   "all sessions revoked",
		Metadata: map[string]string{"scope": "all_sessions"},
	})
	return c.JSON(fiber.Map{"message": "all sessions revoked"})
}

func (h *UserHandler) bumpUserTokenVersion(c fiber.Ctx, userID string, user *model.User) error {
	if user.TokenVersion < 1 {
		user.TokenVersion = 1
	}
	user.TokenVersion++
	if err := h.users.Update(c.Context(), user); err != nil {
		h.auditRevokeAllFailure(c, userID, "token_version_update_failed")
		return jsonError(fiber.StatusInternalServerError, "failed to revoke all sessions")
	}
	return nil
}

func (h *UserHandler) auditRevokeAllFailure(c fiber.Ctx, userID, reason string) {
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:   userID,
		Action:   model.AuditSessionRevoke,
		Resource: "session",
		Status:   "failure",
		Detail:   "revoke all sessions failed",
		Metadata: map[string]string{
			"scope":  "all_sessions",
			"reason": reason,
		},
	})
}
