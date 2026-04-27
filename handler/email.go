package handler

import (
	"context"
	"fmt"
	"html"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
	pointstore "github.com/ysicing/go-template/store/points"

	"github.com/gofiber/fiber/v3"
)

// EmailHandler handles email verification endpoints.
type EmailHandler struct {
	users    *store.UserStore
	settings *store.SettingStore
	audit    *store.AuditLogStore
	points   *pointstore.PointStore
	cache    store.Cache
}

// NewEmailHandler creates an EmailHandler.
func NewEmailHandler(users *store.UserStore, settings *store.SettingStore, audit *store.AuditLogStore, points *pointstore.PointStore, cache store.Cache) *EmailHandler {
	return &EmailHandler{users: users, settings: settings, audit: audit, points: points, cache: cache}
}

const (
	emailVerifyTTL   = 24 * time.Hour
	emailResendTTL   = 2 * time.Minute
	emailVerifyKey   = "email_verify:"
	emailResendKey   = "email_resend:"
	emailMaxRetries  = 3               // Maximum retry attempts
	emailRetryDelay  = 5 * time.Second // Delay between retries
	emailSendTimeout = 30 * time.Second
)

// SendVerificationEmail generates a token and sends a verification email asynchronously.
func (h *EmailHandler) SendVerificationEmail(c fiber.Ctx, user *model.User, baseURL string) error {
	token := store.GenerateRandomToken()
	ephemeral := store.NewEphemeralTokenStore(h.cache)
	if err := ephemeral.IssueString(c.Context(), "verify", "email", token, user.ID, emailVerifyTTL); err != nil {
		return fmt.Errorf("failed to cache verification token: %w", err)
	}

	link := baseURL + "/verify-email?token=" + token
	body := fmt.Sprintf(`<html><body>
<h2>Email Verification</h2>
<p>Hi %s, please click the link below to verify your email address:</p>
<p><a href="%s">%s</a></p>
<p>This link expires in 24 hours.</p>
</body></html>`, htmlEscape(user.Username), link, link)

	// Capture values before launching goroutine to avoid data races on the user pointer.
	userID, userEmail := user.ID, user.Email

	go h.sendVerificationEmailAsync(userID, userEmail, body)
	return nil
}

// SendTestEmail sends a test message using current SMTP settings.
func (h *EmailHandler) SendTestEmail(ctx context.Context, to string) error {
	body := `<html><body>
<h2>SMTP Test Email</h2>
<p>This is a test email from ID service.</p>
</body></html>`
	return h.sendEmailWithContext(ctx, to, "SMTP Test Email", body)
}

// VerifyEmail handles POST /api/auth/verify-email.
func (h *EmailHandler) VerifyEmail(c fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "token is required"})
	}

	ephemeral := store.NewEphemeralTokenStore(h.cache)
	userID, err := ephemeral.ConsumeString(c.Context(), "verify", "email", req.Token)
	if err != nil || userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid or expired token"})
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user not found"})
	}

	if user.EmailVerified {
		return c.JSON(fiber.Map{"message": "email already verified"})
	}

	user.EmailVerified = true
	if err := h.users.Update(c.Context(), user); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to verify email"})
	}

	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: user.ID, Action: model.AuditEmailVerify, Resource: "user",
		ResourceID: user.ID, IP: ip, UserAgent: ua, Status: "success",
	})

	h.handleInviteReward(c.Context(), user, ip, ua)

	return c.JSON(fiber.Map{"message": "email verified"})
}

// ResendVerification handles POST /api/auth/resend-verification.
func (h *EmailHandler) ResendVerification(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	user, err := h.users.GetByID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user not found"})
	}

	if user.EmailVerified {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email already verified"})
	}

	// Rate limit: one resend per 2 minutes
	ok, err := h.cache.SetNX(c.Context(), emailResendKey+userID, "1", emailResendTTL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to process resend request"})
	}
	if !ok {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "please wait before requesting another verification email"})
	}

	baseURL := c.Protocol() + "://" + c.Hostname()
	// SendVerificationEmail now sends asynchronously, so it won't block
	if err := h.SendVerificationEmail(c, user, baseURL); err != nil {
		_ = h.cache.Del(c.Context(), emailResendKey+userID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to queue verification email"})
	}

	ip, ua := GetRealIPAndUA(c)
	_ = writeAudit(c.Context(), h.audit, &model.AuditLog{
		UserID: user.ID, Action: model.AuditEmailResend, Resource: "user",
		ResourceID: user.ID, IP: ip, UserAgent: ua, Status: "success",
	})

	return c.JSON(fiber.Map{"message": "verification email queued for sending"})
}

// htmlEscape escapes HTML special characters to prevent XSS in email templates.
func htmlEscape(s string) string {
	return html.EscapeString(s)
}
