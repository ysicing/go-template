package handler

import (
	"strings"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/validator"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type registerRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}

// Register handles POST /api/auth/register.
func (h *AuthHandler) Register(c fiber.Ctx) error {
	req, err := h.parseRegisterRequest(c)
	if err != nil {
		return finishHandlerError(c, err)
	}

	invitedByUserID, err := h.resolveRegisterInvite(c, req.InviteCode)
	if err != nil {
		return finishHandlerError(c, err)
	}

	user, err := h.buildRegisteredUser(c, req, invitedByUserID)
	if err != nil {
		return finishHandlerError(c, err)
	}
	if err := h.persistRegisteredUser(c, user); err != nil {
		return finishHandlerError(c, err)
	}

	h.recordAudit(c, user.ID, model.AuditRegister, "user", user.ID, "success", "")
	RecordAuthAttempt("register", "success")
	return h.finishRegister(c, user)
}

func (h *AuthHandler) parseRegisterRequest(c fiber.Ctx) (*registerRequest, error) {
	var req registerRequest
	if err := c.Bind().JSON(&req); err != nil {
		return nil, jsonError(fiber.StatusBadRequest, "invalid request body")
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	if err := h.validateRegisterRequest(req); err != nil {
		return nil, err
	}
	return &req, nil
}

func (h *AuthHandler) validateRegisterRequest(req registerRequest) error {
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return jsonError(fiber.StatusBadRequest, "username, email and password are required")
	}
	if len(req.Username) < 3 || len(req.Username) > 32 {
		return jsonError(fiber.StatusBadRequest, "username must be 3-32 characters")
	}
	if !isValidEmail(req.Email) {
		return jsonError(fiber.StatusBadRequest, "invalid email format")
	}
	if err := h.validateRegisterEmailDomain(req.Email); err != nil {
		return err
	}
	if shouldEnforcePasswordPolicy(h.settings) {
		if err := model.ValidatePasswordStrength(req.Password); err != nil {
			return jsonError(fiber.StatusBadRequest, err.Error())
		}
	}
	return nil
}

func (h *AuthHandler) validateRegisterEmailDomain(email string) error {
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

func (h *AuthHandler) resolveRegisterInvite(c fiber.Ctx, inviteCode string) (string, error) {
	if inviteCode == "" {
		return "", nil
	}

	inviter, err := h.users.GetByInviteCode(c.Context(), inviteCode)
	if err != nil || inviter == nil {
		return "", jsonError(fiber.StatusBadRequest, "invalid invite code")
	}
	return inviter.ID, nil
}

func (h *AuthHandler) buildRegisteredUser(c fiber.Ctx, req *registerRequest, invitedByUserID string) (*model.User, error) {
	inviteCode := uuid.NewString()
	if len(inviteCode) > 32 {
		inviteCode = inviteCode[:32]
	}

	user := &model.User{
		Username:        req.Username,
		Email:           req.Email,
		Provider:        "local",
		ProviderID:      req.Username,
		InviteCode:      inviteCode,
		InvitedByUserID: invitedByUserID,
	}
	if invitedByUserID != "" {
		user.InviteIP = GetRealIP(c)
	}
	if err := user.SetPassword(req.Password); err != nil {
		return nil, jsonError(fiber.StatusInternalServerError, "failed to hash password")
	}
	return user, nil
}

func (h *AuthHandler) persistRegisteredUser(c fiber.Ctx, user *model.User) error {
	if err := h.users.Create(c.Context(), user); err != nil {
		RecordAuthAttempt("register", "failure")
		if store.IsUniqueViolation(err) {
			return jsonError(fiber.StatusConflict, "username or email already exists")
		}
		return jsonError(fiber.StatusInternalServerError, "failed to create user")
	}
	return nil
}

func (h *AuthHandler) finishRegister(c fiber.Ctx, user *model.User) error {
	resp := fiber.Map{"user": NewUserResponse(user)}
	emailVerificationRequired := h.settings != nil && h.settings.GetBool(store.SettingEmailVerificationEnabled, false)
	if emailVerificationRequired {
		resp["email_verification_required"] = true
		h.sendRegisterVerificationEmail(c, user)
		return c.Status(fiber.StatusCreated).JSON(resp)
	}

	issuedSession, err := h.sessions.IssueBrowserSession(c.Context(), browserSessionRequest(c, user, h.tokenConfig.RefreshTTL))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate tokens"})
	}

	SetTokenCookies(c, issuedSession.AccessToken, issuedSession.RefreshToken, h.tokenConfig.AccessTTL, h.tokenConfig.RefreshTTL)
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *AuthHandler) sendRegisterVerificationEmail(c fiber.Ctx, user *model.User) {
	if h.emailHandler == nil {
		return
	}
	baseURL := c.Protocol() + "://" + c.Hostname()
	_ = h.emailHandler.SendVerificationEmail(c, user, baseURL)
}
