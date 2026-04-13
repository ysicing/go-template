package handler

import (
	"context"
	"net/mail"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

type emailTester interface {
	SendTestEmail(ctx context.Context, to string) error
}

type AdminSettingHandler struct {
	settings *store.SettingStore
	audit    *store.AuditLogStore
	email    emailTester
}

func NewAdminSettingHandler(settings *store.SettingStore, audit *store.AuditLogStore, email emailTester) *AdminSettingHandler {
	return &AdminSettingHandler{settings: settings, audit: audit, email: email}
}

func (h *AdminSettingHandler) Get(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"register_enabled":           h.settings.GetBool(store.SettingRegisterEnabled, true),
		"password_policy_enabled":    h.settings.GetBool(store.SettingPasswordPolicyEnabled, false),
		"site_title":                 h.settings.Get(store.SettingSiteTitle, ""),
		"cors_origins":               h.settings.Get(store.SettingCORSOrigins, ""),
		"webauthn_rp_id":             h.settings.Get(store.SettingWebAuthnRPID, ""),
		"webauthn_rp_display_name":   h.settings.Get(store.SettingWebAuthnRPDisplay, ""),
		"webauthn_rp_origins":        h.settings.Get(store.SettingWebAuthnRPOrigins, ""),
		"turnstile_site_key":         h.settings.Get(store.SettingTurnstileSiteKey, ""),
		"turnstile_secret_key":       maskSecret(h.settings.Get(store.SettingTurnstileSecretKey, "")),
		"smtp_host":                  h.settings.Get(store.SettingSMTPHost, ""),
		"smtp_port":                  h.settings.Get(store.SettingSMTPPort, "587"),
		"smtp_username":              h.settings.Get(store.SettingSMTPUsername, ""),
		"smtp_password":              maskSecret(h.settings.Get(store.SettingSMTPPassword, "")),
		"smtp_from_address":          h.settings.Get(store.SettingSMTPFromAddress, ""),
		"smtp_tls":                   h.settings.GetBool(store.SettingSMTPTLS, true),
		"email_verification_enabled": h.settings.GetBool(store.SettingEmailVerificationEnabled, false),
		"email_domain_mode":          h.settings.Get(store.SettingEmailDomainMode, "disabled"),
		"email_domain_whitelist":     h.settings.Get(store.SettingEmailDomainWhitelist, ""),
		"email_domain_blacklist":     h.settings.Get(store.SettingEmailDomainBlacklist, ""),
		"invite_reward_enabled":      h.settings.GetBool(store.SettingInviteRewardEnabled, true),
		"invite_reward_min":          h.settings.Get(store.SettingInviteRewardMin, "1"),
		"invite_reward_max":          h.settings.Get(store.SettingInviteRewardMax, "5"),
	})
}

// maskSecret returns a masked version of a secret string for display.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 6 {
		return "***"
	}
	return s[:3] + "***" + s[len(s)-3:]
}

// isMasked returns true if the value looks like a masked secret (contains ***).
func isMasked(s string) bool {
	return strings.Contains(s, "***")
}

// setBoolSetting sets a boolean setting if the value is not nil.
func setBoolSetting(values map[string]string, key string, val *bool) {
	if val != nil {
		values[key] = strconv.FormatBool(*val)
	}
}

// setStringSetting sets a string setting if the value is not nil and not masked.
func setStringSetting(values map[string]string, key string, val *string, skipMasked bool) {
	if val != nil && (!skipMasked || !isMasked(*val)) {
		values[key] = *val
	}
}

func parseInviteRewardValue(v string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, err
	}
	if n < 1 || n > 5 {
		return 0, fiber.ErrBadRequest
	}
	return n, nil
}

func validateInviteRewardSettings(minVal, maxVal *string) error {
	min := 1
	max := 5
	var err error

	if minVal != nil {
		min, err = parseInviteRewardValue(*minVal)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invite_reward_min must be an integer between 1 and 5")
		}
	}
	if maxVal != nil {
		max, err = parseInviteRewardValue(*maxVal)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invite_reward_max must be an integer between 1 and 5")
		}
	}
	if min > max {
		return fiber.NewError(fiber.StatusBadRequest, "invite_reward_min must be less than or equal to invite_reward_max")
	}
	return nil
}

func (h *AdminSettingHandler) TestEmail(c fiber.Ctx) error {
	if h.email == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "email service unavailable"})
	}

	var req struct {
		To string `json:"to"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	to := strings.TrimSpace(req.To)
	if to == "" {
		to = h.settings.Get(store.SettingSMTPFromAddress, "")
	}
	if to == "" {
		to = h.settings.Get(store.SettingSMTPUsername, "")
	}
	if to == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "test recipient is required"})
	}
	if _, err := mail.ParseAddress(to); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid email address"})
	}

	if err := h.email.SendTestEmail(c.Context(), to); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	settingUserID, _ := c.Locals("user_id").(string)
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:   settingUserID,
		Action:   model.AuditSettingUpdate,
		Resource: "setting_email_test",
		Status:   "success",
		Detail:   "test email sent",
		Metadata: map[string]string{
			"recipient": to,
		},
	})

	return c.JSON(fiber.Map{"message": "test email sent"})
}

func (h *AdminSettingHandler) Update(c fiber.Ctx) error {
	var req struct {
		RegisterEnabled          *bool   `json:"register_enabled"`
		PasswordPolicyEnabled    *bool   `json:"password_policy_enabled"`
		SiteTitle                *string `json:"site_title"`
		CORSOrigins              *string `json:"cors_origins"`
		WebAuthnRPID             *string `json:"webauthn_rp_id"`
		WebAuthnRPDisplay        *string `json:"webauthn_rp_display_name"`
		WebAuthnRPOrigins        *string `json:"webauthn_rp_origins"`
		TurnstileSiteKey         *string `json:"turnstile_site_key"`
		TurnstileSecret          *string `json:"turnstile_secret_key"`
		SMTPHost                 *string `json:"smtp_host"`
		SMTPPort                 *string `json:"smtp_port"`
		SMTPUsername             *string `json:"smtp_username"`
		SMTPPassword             *string `json:"smtp_password"`
		SMTPFromAddress          *string `json:"smtp_from_address"`
		SMTPTLS                  *bool   `json:"smtp_tls"`
		EmailVerificationEnabled *bool   `json:"email_verification_enabled"`
		EmailDomainMode          *string `json:"email_domain_mode"`
		EmailDomainWhitelist     *string `json:"email_domain_whitelist"`
		EmailDomainBlacklist     *string `json:"email_domain_blacklist"`
		InviteRewardEnabled      *bool   `json:"invite_reward_enabled"`
		InviteRewardMin          *string `json:"invite_reward_min"`
		InviteRewardMax          *string `json:"invite_reward_max"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	ctx := c.Context()
	pendingValues := make(map[string]string)

	if err := validateInviteRewardSettings(req.InviteRewardMin, req.InviteRewardMax); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Boolean settings
	setBoolSetting(pendingValues, store.SettingRegisterEnabled, req.RegisterEnabled)
	setBoolSetting(pendingValues, store.SettingPasswordPolicyEnabled, req.PasswordPolicyEnabled)
	setBoolSetting(pendingValues, store.SettingSMTPTLS, req.SMTPTLS)
	setBoolSetting(pendingValues, store.SettingEmailVerificationEnabled, req.EmailVerificationEnabled)
	setBoolSetting(pendingValues, store.SettingInviteRewardEnabled, req.InviteRewardEnabled)

	// String settings (no masking)
	setStringSetting(pendingValues, store.SettingSiteTitle, req.SiteTitle, false)
	setStringSetting(pendingValues, store.SettingCORSOrigins, req.CORSOrigins, false)
	setStringSetting(pendingValues, store.SettingWebAuthnRPID, req.WebAuthnRPID, false)
	setStringSetting(pendingValues, store.SettingWebAuthnRPDisplay, req.WebAuthnRPDisplay, false)
	setStringSetting(pendingValues, store.SettingWebAuthnRPOrigins, req.WebAuthnRPOrigins, false)
	setStringSetting(pendingValues, store.SettingTurnstileSiteKey, req.TurnstileSiteKey, false)
	setStringSetting(pendingValues, store.SettingSMTPHost, req.SMTPHost, false)
	setStringSetting(pendingValues, store.SettingSMTPPort, req.SMTPPort, false)
	setStringSetting(pendingValues, store.SettingSMTPUsername, req.SMTPUsername, false)
	setStringSetting(pendingValues, store.SettingSMTPFromAddress, req.SMTPFromAddress, false)
	setStringSetting(pendingValues, store.SettingEmailDomainMode, req.EmailDomainMode, false)
	setStringSetting(pendingValues, store.SettingEmailDomainWhitelist, req.EmailDomainWhitelist, false)
	setStringSetting(pendingValues, store.SettingEmailDomainBlacklist, req.EmailDomainBlacklist, false)
	setStringSetting(pendingValues, store.SettingInviteRewardMin, req.InviteRewardMin, false)
	setStringSetting(pendingValues, store.SettingInviteRewardMax, req.InviteRewardMax, false)

	// Secret settings (skip if masked)
	setStringSetting(pendingValues, store.SettingTurnstileSecretKey, req.TurnstileSecret, true)
	setStringSetting(pendingValues, store.SettingSMTPPassword, req.SMTPPassword, true)

	if err := h.settings.SetMany(ctx, pendingValues); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update settings"})
	}

	settingUserID, _ := c.Locals("user_id").(string)
	_ = recordAuditFromFiber(c, h.audit, AuditEvent{
		UserID:   settingUserID,
		Action:   model.AuditSettingUpdate,
		Resource: "setting",
		Status:   "success",
		Detail:   "settings updated",
	})

	return c.JSON(fiber.Map{"message": "settings updated"})
}
