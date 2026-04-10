package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/ysicing/go-template/internal/mailer"
	"github.com/ysicing/go-template/internal/system"
	"github.com/ysicing/go-template/internal/user"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials        = errors.New("invalid credentials")
	ErrInvalidRefreshToken       = errors.New("invalid refresh token")
	ErrInvalidPasswordResetToken = errors.New("invalid password reset token")
	ErrPasswordResetUnavailable  = errors.New("password reset unavailable")
)

type Service struct {
	db         *gorm.DB
	tokens     *TokenManager
	mailSender mailer.Sender
	now        func() time.Time
}

type Option func(*Service)

func WithPasswordResetSender(sender mailer.Sender) Option {
	return func(service *Service) {
		if sender != nil {
			service.mailSender = sender
		}
	}
}

func WithNowFunc(now func() time.Time) Option {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

func NewService(db *gorm.DB, tokens *TokenManager, options ...Option) *Service {
	service := &Service{
		db:         db,
		tokens:     tokens,
		mailSender: mailer.NewSMTPClient(),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) Login(identifier string, password string) (*user.User, TokenPair, error) {
	var account user.User
	if err := s.db.Where("username = ? OR email = ?", identifier, identifier).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, TokenPair{}, ErrInvalidCredentials
		}
		return nil, TokenPair{}, err
	}
	if err := CheckPassword(account.PasswordHash, password); err != nil {
		return nil, TokenPair{}, ErrInvalidCredentials
	}
	if account.Status != "active" {
		return nil, TokenPair{}, ErrInvalidCredentials
	}

	now := time.Now().UTC()
	account.LastLoginAt = &now
	if err := s.db.Save(&account).Error; err != nil {
		return nil, TokenPair{}, err
	}

	pair, err := s.tokens.Issue(account.ID, string(account.Role))
	if err != nil {
		return nil, TokenPair{}, err
	}

	return &account, pair, nil
}

func (s *Service) Refresh(refreshToken string) (TokenPair, error) {
	claims, err := s.tokens.ParseRefresh(refreshToken)
	if err != nil {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	return s.tokens.Issue(claims.UserID, claims.Role)
}

func (s *Service) CurrentUser(userID uint) (*user.User, error) {
	var account user.User
	if err := s.db.First(&account, userID).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	settings, err := system.LoadMailSettings(s.db)
	if err != nil {
		return err
	}
	if !system.MailSettingsConfigured(settings) {
		return ErrPasswordResetUnavailable
	}

	normalizedEmail := strings.TrimSpace(email)
	if normalizedEmail == "" {
		return nil
	}

	var account user.User
	if err := s.db.Where("email = ?", normalizedEmail).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if account.Status != "active" {
		return nil
	}

	token, tokenHash, err := issuePasswordResetToken()
	if err != nil {
		return err
	}

	expiresAt := s.now().Add(passwordResetTTL)
	record := PasswordResetToken{
		UserID:    account.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", account.ID).Delete(&PasswordResetToken{}).Error; err != nil {
			return err
		}
		return tx.Create(&record).Error
	}); err != nil {
		return err
	}

	resetURL := settings.ResetBaseURL + "/reset-password?token=" + url.QueryEscape(token)
	return s.mailSender.SendPasswordReset(&mailer.SMTPConfig{
		Enabled:  settings.Enabled,
		Host:     settings.SMTPHost,
		Port:     settings.SMTPPort,
		Username: settings.Username,
		Password: settings.Password,
		From:     settings.From,
	}, mailer.PasswordResetMessage{
		SiteName:  settings.SiteName,
		ToEmail:   account.Email,
		ToName:    account.Username,
		ResetURL:  resetURL,
		ExpiresAt: expiresAt,
	})
}

func (s *Service) ResetPasswordByToken(ctx context.Context, token string, input user.ResetPasswordInput) error {
	_ = ctx
	normalizedToken := strings.TrimSpace(token)
	if normalizedToken == "" {
		return ErrInvalidPasswordResetToken
	}

	var record PasswordResetToken
	if err := s.db.Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", hashPasswordResetToken(normalizedToken), s.now()).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidPasswordResetToken
		}
		return err
	}

	userService := user.NewService(s.db)
	if err := userService.ResetPassword(record.UserID, record.UserID, input); err != nil {
		return err
	}

	now := s.now()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&record).Update("used_at", &now).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND id <> ?", record.UserID, record.ID).Delete(&PasswordResetToken{}).Error
	})
}
