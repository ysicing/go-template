package handler

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"
)

func (h *EmailHandler) sendVerificationEmailAsync(userID, userEmail, body string) {
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

			if attempt > 1 {
				RecordEmailRetry()
			}
			if !waitEmailRetryDelay(ctx, attempt) {
				break
			}
			continue
		}

		logger.L.Info().
			Str("user_id", userID).
			Str("email", userEmail).
			Int("attempt", attempt).
			Msg("verification email sent successfully")
		RecordEmailSent("verification", "success")
		return
	}

	logger.L.Error().
		Err(lastErr).
		Str("user_id", userID).
		Str("email", userEmail).
		Int("attempts", emailMaxRetries).
		Msg("failed to send verification email after all retries")
	RecordEmailSent("verification", "failure")
}

func waitEmailRetryDelay(ctx context.Context, attempt int) bool {
	if ctx.Err() != nil || attempt >= emailMaxRetries {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(emailRetryDelay):
		return true
	}
}

// sendEmailWithContext sends an email with a context for timeout control.
func (h *EmailHandler) sendEmailWithContext(ctx context.Context, to, subject, body string) error {
	cfg, err := h.loadSMTPConfig()
	if err != nil {
		return err
	}
	msg := buildEmailMessage(cfg.from, to, subject, body)
	done := make(chan error, 1)

	go func() {
		done <- h.sendEmailSync(cfg.addr, cfg.host, cfg.port, cfg.username, cfg.password, cfg.from, to, msg, cfg.useTLS)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("email send timeout: %w", ctx.Err())
	case err := <-done:
		return err
	}
}

type smtpConfig struct {
	addr     string
	host     string
	port     int
	username string
	password string
	from     string
	useTLS   bool
}

func (h *EmailHandler) loadSMTPConfig() (smtpConfig, error) {
	host := h.settings.Get(store.SettingSMTPHost, "")
	if host == "" {
		return smtpConfig{}, fmt.Errorf("SMTP not configured")
	}
	port, _ := strconv.Atoi(h.settings.Get(store.SettingSMTPPort, "587"))
	username := h.settings.Get(store.SettingSMTPUsername, "")
	password := h.settings.Get(store.SettingSMTPPassword, "")
	from := h.settings.Get(store.SettingSMTPFromAddress, "")
	if from == "" {
		from = username
	}
	return smtpConfig{
		addr:     net.JoinHostPort(host, strconv.Itoa(port)),
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     sanitizeEmailHeader(from),
		useTLS:   h.settings.GetBool(store.SettingSMTPTLS, true),
	}, nil
}

func buildEmailMessage(from, to, subject, body string) string {
	sanitizedTo := sanitizeEmailHeader(to)
	sanitizedSubject := sanitizeEmailHeader(subject)
	return "From: " + from + "\r\n" +
		"To: " + sanitizedTo + "\r\n" +
		"Subject: " + sanitizedSubject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" + body
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
		return sendEmailWithImplicitTLS(addr, host, username, password, from, to, msg)
	}
	return sendEmailWithStartTLS(addr, host, username, password, from, to, msg, useTLS)
}

func sendEmailWithImplicitTLS(addr, host, username, password, from, to, msg string) error {
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
	return sendEmailData(c, host, username, password, from, to, msg)
}

func sendEmailWithStartTLS(addr, host, username, password, from, to, msg string, useTLS bool) error {
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
	return sendEmailData(c, host, username, password, from, to, msg)
}

func sendEmailData(c *smtp.Client, host, username, password, from, to, msg string) error {
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
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	return w.Close()
}
