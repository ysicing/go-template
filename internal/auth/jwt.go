package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID uint   `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type TokenManager struct {
	issuer     string
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(issuer string, secret string, accessTTL time.Duration, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		issuer:     issuer,
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (m *TokenManager) Issue(userID uint, role string) (TokenPair, error) {
	now := time.Now()

	access, err := m.sign(userID, role, "access", now.Add(m.accessTTL), now)
	if err != nil {
		return TokenPair{}, err
	}

	refresh, err := m.sign(userID, role, "refresh", now.Add(m.refreshTTL), now)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (m *TokenManager) Refresh(refreshToken string) (TokenPair, error) {
	claims, err := m.ParseRefresh(refreshToken)
	if err != nil {
		return TokenPair{}, err
	}

	return m.Issue(claims.UserID, claims.Role)
}

func (m *TokenManager) ParseAccess(token string) (*Claims, error) {
	return m.parse(token, "access")
}

func (m *TokenManager) ParseRefresh(token string) (*Claims, error) {
	return m.parse(token, "refresh")
}

func (m *TokenManager) sign(userID uint, role string, subject string, expiresAt time.Time, issuedAt time.Time) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
		},
	}).SignedString(m.secret)
}

func (m *TokenManager) parse(token string, subject string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(_ *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}
	if claims.Subject != subject {
		return nil, errors.New("invalid token subject")
	}

	return claims, nil
}

