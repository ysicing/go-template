package auth

import (
	"errors"
	"time"

	"github.com/ysicing/go-template/internal/user"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

type Service struct {
	db     *gorm.DB
	tokens *TokenManager
}

func NewService(db *gorm.DB, tokens *TokenManager) *Service {
	return &Service{db: db, tokens: tokens}
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
