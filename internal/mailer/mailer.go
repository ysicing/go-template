package mailer

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

type SMTPConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type PasswordResetMessage struct {
	SiteName  string
	ToEmail   string
	ToName    string
	ResetURL  string
	ExpiresAt time.Time
}

type Sender interface {
	SendPasswordReset(config *SMTPConfig, message PasswordResetMessage) error
}

type SMTPClient struct{}

func NewSMTPClient() *SMTPClient {
	return &SMTPClient{}
}

func (c *SMTPClient) SendPasswordReset(config *SMTPConfig, message PasswordResetMessage) error {
	if config == nil {
		return fmt.Errorf("smtp config is required")
	}
	if strings.TrimSpace(config.Host) == "" || config.Port <= 0 {
		return fmt.Errorf("smtp host and port are required")
	}
	if strings.TrimSpace(config.From) == "" {
		return fmt.Errorf("smtp from is required")
	}
	if strings.TrimSpace(message.ToEmail) == "" {
		return fmt.Errorf("recipient email is required")
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	subject := fmt.Sprintf("[%s] Reset your password", normalizeSiteName(message.SiteName))
	body := fmt.Sprintf(
		"Hello %s,\r\n\r\nUse the link below to reset your password:\r\n%s\r\n\r\nThis link expires at %s.\r\n",
		normalizeRecipientName(message.ToName, message.ToEmail),
		message.ResetURL,
		message.ExpiresAt.UTC().Format(time.RFC3339),
	)
	payload := []byte(
		"To: " + message.ToEmail + "\r\n" +
			"From: " + config.From + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n" +
			body,
	)

	var auth smtp.Auth
	if strings.TrimSpace(config.Username) != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	return smtp.SendMail(addr, auth, config.From, []string{message.ToEmail}, payload)
}

func normalizeSiteName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return "Go Template"
	}
	return name
}

func normalizeRecipientName(name string, email string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(email)
}
