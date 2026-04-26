package handler

import (
	"encoding/json"

	"github.com/ysicing/go-template/model"

	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
)

// TOTPEnable handles POST /api/mfa/totp/enable.
func (h *MFAHandler) TOTPEnable(c fiber.Ctx) error {
	userID, secret, err := h.loadPendingTOTPSecret(c)
	if err != nil {
		return finishHandlerError(c, err)
	}
	backupCodes, err := h.persistTOTPEnable(c, userID, secret)
	if err != nil {
		return finishHandlerError(c, err)
	}
	return c.JSON(fiber.Map{"message": "TOTP enabled", "backup_codes": backupCodes})
}

func (h *MFAHandler) loadPendingTOTPSecret(c fiber.Ctx) (string, string, error) {
	userID, _ := c.Locals("user_id").(string)
	var req struct {
		Code string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.Code == "" {
		return "", "", jsonError(fiber.StatusBadRequest, "code is required")
	}
	secret, err := h.cache.Get(c.Context(), "totp_setup:"+userID)
	if err != nil || secret == "" {
		return "", "", jsonError(fiber.StatusBadRequest, "no pending TOTP setup, call /api/mfa/totp/setup first")
	}
	if !totp.Validate(req.Code, secret) {
		return "", "", jsonError(fiber.StatusUnauthorized, "invalid TOTP code")
	}
	return userID, secret, nil
}

func (h *MFAHandler) persistTOTPEnable(c fiber.Ctx, userID, secret string) ([]string, error) {
	backupCodes := generateBackupCodes(10)
	hashedCodes := hashBackupCodes(backupCodes)
	codesJSON, err := json.Marshal(hashedCodes)
	if err != nil {
		return nil, jsonError(fiber.StatusInternalServerError, "failed to encode backup codes")
	}

	cfg := &model.MFAConfig{
		UserID:      userID,
		TOTPSecret:  secret,
		TOTPEnabled: true,
		BackupCodes: string(codesJSON),
	}
	if err := h.mfa.Upsert(c.Context(), cfg); err != nil {
		return nil, jsonError(fiber.StatusInternalServerError, "failed to enable TOTP")
	}

	_ = h.cache.Del(c.Context(), "totp_setup:"+userID)
	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditMFAEnable, Resource: "mfa",
		IP: ip, UserAgent: ua, Status: "success",
	})
	return backupCodes, nil
}

// TOTPDisable handles POST /api/mfa/totp/disable.
func (h *MFAHandler) TOTPDisable(c fiber.Ctx) error {
	userID, err := h.verifyDisableTOTPRequest(c)
	if err != nil {
		return finishHandlerError(c, err)
	}
	if err := h.mfa.Delete(c.Context(), userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to disable TOTP"})
	}

	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditMFADisable, Resource: "mfa",
		IP: ip, UserAgent: ua, Status: "success",
	})
	return c.JSON(fiber.Map{"message": "TOTP disabled"})
}

func (h *MFAHandler) verifyDisableTOTPRequest(c fiber.Ctx) (string, error) {
	userID, _ := c.Locals("user_id").(string)
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return "", jsonError(fiber.StatusBadRequest, "invalid request body")
	}
	if isAccountLocked(c.Context(), h.cache, userID) {
		return "", jsonError(fiber.StatusTooManyRequests, "too many attempts, try again later")
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return "", jsonError(fiber.StatusNotFound, "user not found")
	}
	if user.PasswordHash == "" {
		return userID, h.verifySocialUserTOTPDisable(c, userID, req.Code)
	}
	return userID, h.verifyLocalUserTOTPDisable(c, userID, user, req.Password)
}

func (h *MFAHandler) verifySocialUserTOTPDisable(c fiber.Ctx, userID, code string) error {
	if code == "" {
		return jsonError(fiber.StatusBadRequest, "totp code is required")
	}
	cfg, err := h.mfa.GetByUserID(c.Context(), userID)
	if err != nil || !cfg.TOTPEnabled {
		return jsonError(fiber.StatusBadRequest, "TOTP not enabled")
	}
	if !totp.Validate(code, cfg.TOTPSecret) {
		recordFailedAuthAttempt(c.Context(), h.cache, userID)
		return jsonError(fiber.StatusUnauthorized, "invalid TOTP code")
	}
	return nil
}

func (h *MFAHandler) verifyLocalUserTOTPDisable(c fiber.Ctx, userID string, user *model.User, password string) error {
	if password == "" {
		return jsonError(fiber.StatusBadRequest, "password is required")
	}
	if !user.CheckPassword(password) {
		recordFailedAuthAttempt(c.Context(), h.cache, userID)
		return jsonError(fiber.StatusUnauthorized, "invalid password")
	}
	return nil
}

// RegenerateBackupCodes handles POST /api/mfa/backup-codes/regenerate.
func (h *MFAHandler) RegenerateBackupCodes(c fiber.Ctx) error {
	userID, cfg, err := h.verifyBackupCodeRegeneration(c)
	if err != nil {
		return finishHandlerError(c, err)
	}

	backupCodes, err := h.replaceBackupCodes(c, cfg)
	if err != nil {
		return finishHandlerError(c, err)
	}

	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: userID, Action: model.AuditMFABackupRegenerate, Resource: "mfa",
		IP: ip, UserAgent: ua, Status: "success",
	})
	return c.JSON(fiber.Map{"backup_codes": backupCodes})
}

func (h *MFAHandler) verifyBackupCodeRegeneration(c fiber.Ctx) (string, *model.MFAConfig, error) {
	userID, _ := c.Locals("user_id").(string)
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return "", nil, jsonError(fiber.StatusBadRequest, "invalid request body")
	}
	if isAccountLocked(c.Context(), h.cache, userID) {
		return "", nil, jsonError(fiber.StatusTooManyRequests, "too many attempts, try again later")
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return "", nil, jsonError(fiber.StatusNotFound, "user not found")
	}
	cfg, err := h.mfa.GetByUserID(c.Context(), userID)
	if err != nil || !cfg.TOTPEnabled {
		return "", nil, jsonError(fiber.StatusBadRequest, "TOTP not enabled")
	}

	if user.PasswordHash == "" {
		if err := h.verifySocialUserTOTPDisable(c, userID, req.Code); err != nil {
			return "", nil, err
		}
		return userID, cfg, nil
	}
	if err := h.verifyLocalUserTOTPDisable(c, userID, user, req.Password); err != nil {
		return "", nil, err
	}
	return userID, cfg, nil
}

func (h *MFAHandler) replaceBackupCodes(c fiber.Ctx, cfg *model.MFAConfig) ([]string, error) {
	backupCodes := generateBackupCodes(10)
	hashedCodes := hashBackupCodes(backupCodes)
	codesJSON, err := json.Marshal(hashedCodes)
	if err != nil {
		return nil, jsonError(fiber.StatusInternalServerError, "failed to encode backup codes")
	}
	cfg.BackupCodes = string(codesJSON)
	if err := h.mfa.Upsert(c.Context(), cfg); err != nil {
		return nil, jsonError(fiber.StatusInternalServerError, "failed to regenerate backup codes")
	}
	return backupCodes, nil
}
