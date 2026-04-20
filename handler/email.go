package handler

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"math/rand/v2"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

// EmailHandler handles email verification endpoints.
type EmailHandler struct {
	users    *store.UserStore
	settings *store.SettingStore
	audit    *store.AuditLogStore
	points   *store.PointStore
	cache    store.Cache
}

// NewEmailHandler creates an EmailHandler.
func NewEmailHandler(users *store.UserStore, settings *store.SettingStore, audit *store.AuditLogStore, points *store.PointStore, cache store.Cache) *EmailHandler {
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

	// Send email asynchronously with retry mechanism
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), emailSendTimeout)
		defer cancel()

		var lastErr error
		for attempt := 1; attempt <= emailMaxRetries; attempt++ {
			if err := h.sendEmailWithContext(ctx, userEmail, "Verify your email address", body); err != nil {
				lastErr = err
				logger.L.Warn().
					Err(err).
					Str("user_id", userID).
					Str("email", userEmail).
					Int("attempt", attempt).
					Int("max_retries", emailMaxRetries).
					Msg("failed to send verification email, will retry")

				// Record retry metric (except on first attempt)
				if attempt > 1 {
					RecordEmailRetry()
				}

				// Don't retry if context is cancelled
				if ctx.Err() != nil {
					break
				}

				// Wait before retry (except on last attempt)
				if attempt < emailMaxRetries {
					select {
					case <-ctx.Done():
						break
					case <-time.After(emailRetryDelay):
						continue
					}
				}
			} else {
				// Success
				logger.L.Info().
					Str("user_id", userID).
					Str("email", userEmail).
					Int("attempt", attempt).
					Msg("verification email sent successfully")
				RecordEmailSent("verification", "success")
				return
			}
		}

		// All retries failed
		logger.L.Error().
			Err(lastErr).
			Str("user_id", userID).
			Str("email", userEmail).
			Int("attempts", emailMaxRetries).
			Msg("failed to send verification email after all retries")
		RecordEmailSent("verification", "failure")
	}()

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

// sendEmailWithContext sends an email with a context for timeout control.
func (h *EmailHandler) sendEmailWithContext(ctx context.Context, to, subject, body string) error {
	host := h.settings.Get(store.SettingSMTPHost, "")
	if host == "" {
		return fmt.Errorf("SMTP not configured")
	}
	port, _ := strconv.Atoi(h.settings.Get(store.SettingSMTPPort, "587"))
	username := h.settings.Get(store.SettingSMTPUsername, "")
	password := h.settings.Get(store.SettingSMTPPassword, "")
	from := h.settings.Get(store.SettingSMTPFromAddress, "")
	useTLS := h.settings.GetBool(store.SettingSMTPTLS, true)

	if from == "" {
		from = username
	}

	// Sanitize subject to prevent header injection
	subject = sanitizeEmailHeader(subject)
	to = sanitizeEmailHeader(to)
	from = sanitizeEmailHeader(from)

	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" + body

	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// Create a channel to signal completion
	done := make(chan error, 1)

	go func() {
		done <- h.sendEmailSync(addr, host, port, username, password, from, to, msg, useTLS)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("email send timeout: %w", ctx.Err())
	case err := <-done:
		return err
	}
}

// sanitizeEmailHeader removes CR/LF characters to prevent header injection.
func sanitizeEmailHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// sendEmailSync performs the actual SMTP send operation.
func (h *EmailHandler) sendEmailSync(addr, host string, port int, username, password, from, to, msg string, useTLS bool) error {
	if port == 465 {
		// Implicit TLS (SMTPS)
		tlsConfig := &tls.Config{ServerName: host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("tls dial: %w", err)
		}
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("smtp client: %w", err)
		}
		defer c.Close()
		if username != "" {
			if err := c.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
		if err := c.Mail(from); err != nil {
			return err
		}
		if err := c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(msg))
		if err != nil {
			return err
		}
		return w.Close()
	}

	// Port 587 or 25: STARTTLS
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer c.Close()
	if useTLS {
		if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if username != "" {
		if err := c.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	return w.Close()
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

func (h *EmailHandler) handleInviteReward(ctx context.Context, invitee *model.User, requestIP, userAgent string) {
	if invitee.InvitedByUserID == "" {
		return
	}
	if !h.settings.GetBool(store.SettingInviteRewardEnabled, true) {
		return
	}

	if invitee.InviteIP != "" && requestIP != "" && invitee.InviteIP == requestIP {
		_ = writeAudit(ctx, h.audit, &model.AuditLog{
			UserID:     invitee.InvitedByUserID,
			Action:     model.AuditInviteRewardSkipped,
			Resource:   "user",
			ResourceID: invitee.ID,
			IP:         requestIP,
			UserAgent:  userAgent,
			Status:     "success",
			Detail:     "invite reward skipped due to same ip",
		})
		return
	}

	minReward := parseInviteRewardBound(h.settings.Get(store.SettingInviteRewardMin, "1"), 1)
	maxReward := parseInviteRewardBound(h.settings.Get(store.SettingInviteRewardMax, "5"), 5)
	if minReward > maxReward {
		minReward, maxReward = maxReward, minReward
	}

	reward := minReward
	if maxReward > minReward {
		reward = minReward + rand.Int64N(maxReward-minReward+1)
	}

	if err := h.points.AddPoints(ctx, invitee.InvitedByUserID, model.PointTypeFree, reward, model.PointKindInviteReward, "invite reward", ""); err != nil {
		logger.L.Warn().
			Err(err).
			Str("inviter_id", invitee.InvitedByUserID).
			Str("invitee_id", invitee.ID).
			Msg("failed to grant invite reward")
		return
	}

	_ = writeAudit(ctx, h.audit, &model.AuditLog{
		UserID:     invitee.InvitedByUserID,
		Action:     model.AuditInviteRewardGranted,
		Resource:   "user",
		ResourceID: invitee.ID,
		IP:         requestIP,
		UserAgent:  userAgent,
		Status:     "success",
		Detail:     fmt.Sprintf("granted %d points", reward),
	})
}

func parseInviteRewardBound(raw string, fallback int64) int64 {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	if v < 1 {
		return 1
	}
	if v > 5 {
		return 5
	}
	return v
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
