package service

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

type TokenConfig struct {
	Secret        string
	Issuer        string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	RememberMeTTL time.Duration
}

type refreshTokenStore interface {
	Create(ctx context.Context, rt *model.APIRefreshToken) error
}

type sessionClaims struct {
	UserID       string   `json:"user_id"`
	IsAdmin      bool     `json:"is_admin"`
	Permissions  []string `json:"permissions,omitempty"`
	TokenVersion int64    `json:"token_version"`
	TokenType    string   `json:"token_type"`
	jwt.RegisteredClaims
}

type SessionRequest struct {
	User       *model.User
	IP         string
	UserAgent  string
	RefreshTTL time.Duration
	Family     string
}

type IssuedSession struct {
	AccessToken  string
	RefreshToken string
	Family       string
}

type SessionService struct {
	refreshTokens refreshTokenStore
	tokens        TokenConfig
}

func NewSessionService(refreshTokens refreshTokenStore, tokens TokenConfig) *SessionService {
	return &SessionService{refreshTokens: refreshTokens, tokens: tokens}
}

func (s *SessionService) IssueBrowserSession(ctx context.Context, req SessionRequest) (*IssuedSession, error) {
	family := req.Family
	if family == "" {
		family = uuid.NewString()
	}
	refreshTTL := req.RefreshTTL
	if refreshTTL == 0 {
		refreshTTL = s.tokens.RefreshTTL
	}

	accessToken, err := s.generateAccessToken(req.User)
	if err != nil {
		return nil, err
	}
	refreshToken := uuid.NewString()
	if err := s.refreshTokens.Create(ctx, &model.APIRefreshToken{
		UserID:     req.User.ID,
		TokenHash:  store.HashToken(refreshToken),
		Family:     family,
		ExpiresAt:  time.Now().Add(refreshTTL),
		IP:         req.IP,
		UserAgent:  req.UserAgent,
		LastUsedAt: time.Now(),
	}); err != nil {
		return nil, err
	}

	return &IssuedSession{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Family:       family,
	}, nil
}

func (s *SessionService) RotateBrowserSession(ctx context.Context, req SessionRequest) (*IssuedSession, error) {
	return s.IssueBrowserSession(ctx, req)
}

func (s *SessionService) generateAccessToken(user *model.User) (string, error) {
	now := time.Now()
	tokenVersion := user.TokenVersion
	if tokenVersion < 1 {
		tokenVersion = 1
	}
	claims := sessionClaims{
		UserID:       user.ID,
		IsAdmin:      user.IsAdmin,
		Permissions:  user.PermissionList(),
		TokenVersion: tokenVersion,
		TokenType:    "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokens.AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   user.ID,
			Issuer:    s.tokens.Issuer,
			Audience:  jwt.ClaimStrings{"id-api"},
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.tokens.Secret))
}
