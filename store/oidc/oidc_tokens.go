package oidcstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"
	rootstore "github.com/ysicing/go-template/store"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

func refreshTokenFromDB(t *model.Token) *oidcRefreshToken {
	subjectID := t.SubjectID
	if subjectID == "" {
		subjectID = t.UserID
	}
	return &oidcRefreshToken{
		id:       t.ID,
		token:    t.RefreshToken,
		userID:   subjectID,
		clientID: t.ClientID,
		scopes:   rootstore.SplitTrimmed(t.Scopes),
		authTime: t.CreatedAt,
		expiry:   t.ExpiresAt,
	}
}

func (s *OIDCStorage) CreateAccessToken(ctx context.Context, req op.TokenRequest) (string, time.Time, error) {
	tokenID := uuid.New().String()
	expiry := time.Now().Add(s.accessTokenTTL)
	clientID := ""
	if aud := req.GetAudience(); len(aud) > 0 {
		clientID = aud[0]
	}
	subjectID := req.GetSubject()
	t := model.Token{
		TokenID:     tokenID,
		UserID:      subjectID,
		SubjectType: "user",
		SubjectID:   subjectID,
		ClientID:    clientID,
		Scopes:      strings.Join(req.GetScopes(), ","),
		TokenType:   "access",
		ExpiresAt:   expiry,
	}
	if err := s.db.WithContext(ctx).Create(&t).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("create access token: %w", err)
	}
	return tokenID, expiry, nil
}

func (s *OIDCStorage) CreateAccessAndRefreshTokens(ctx context.Context, req op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	tokenID := uuid.New().String()
	expiry := time.Now().Add(s.accessTokenTTL)

	clientID := ""
	if aud := req.GetAudience(); len(aud) > 0 {
		clientID = aud[0]
	}
	scopes := strings.Join(req.GetScopes(), ",")
	subjectID := req.GetSubject()

	accessToken := model.Token{
		TokenID:     tokenID,
		UserID:      subjectID,
		SubjectType: "user",
		SubjectID:   subjectID,
		ClientID:    clientID,
		Scopes:      scopes,
		TokenType:   "access",
		ExpiresAt:   expiry,
	}
	refreshTokenStr := uuid.New().String()
	refreshToken := model.Token{
		TokenID:      uuid.New().String(),
		UserID:       subjectID,
		SubjectType:  "user",
		SubjectID:    subjectID,
		ClientID:     clientID,
		Scopes:       scopes,
		TokenType:    "refresh",
		RefreshToken: refreshTokenStr,
		ExpiresAt:    time.Now().Add(s.refreshTokenTTL),
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if currentRefreshToken != "" {
			if err := tx.Unscoped().Where("refresh_token = ? AND token_type = ?", currentRefreshToken, "refresh").Delete(&model.Token{}).Error; err != nil {
				return fmt.Errorf("revoke old refresh token: %w", err)
			}
		}
		if err := tx.Create(&accessToken).Error; err != nil {
			return fmt.Errorf("create access token: %w", err)
		}
		if err := tx.Create(&refreshToken).Error; err != nil {
			return fmt.Errorf("create refresh token: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", "", time.Time{}, err
	}
	return tokenID, refreshTokenStr, expiry, nil
}

func (s *OIDCStorage) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	var t model.Token
	if err := s.db.WithContext(ctx).Where("refresh_token = ? AND token_type = ? AND revoked = false", refreshToken, "refresh").First(&t).Error; err != nil {
		return nil, errors.New("refresh token not found")
	}
	if time.Now().After(t.ExpiresAt) {
		s.db.WithContext(ctx).Where("id = ?", t.ID).Delete(&model.Token{})
		return nil, errors.New("refresh token expired")
	}
	return refreshTokenFromDB(&t), nil
}

func (s *OIDCStorage) TerminateSession(ctx context.Context, userID, clientID string) error {
	q := s.db.WithContext(ctx).Unscoped().Where("user_id = ?", userID)
	if clientID != "" {
		q = q.Where("client_id = ?", clientID)
	}
	return q.Delete(&model.Token{}).Error
}

func (s *OIDCStorage) RevokeToken(ctx context.Context, tokenOrTokenID, userID, clientID string) *oidc.Error {
	query := s.db.WithContext(ctx).Where("token_id = ? OR refresh_token = ?", tokenOrTokenID, tokenOrTokenID)
	if clientID != "" {
		query = query.Where("client_id = ?", clientID)
	}

	var token model.Token
	if err := query.First(&token).Error; err != nil {
		return nil
	}

	_ = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Delete(&model.Token{}, "id = ?", token.ID).Error; err != nil {
			return err
		}
		return tx.Create(&model.AuditLog{
			Action:     model.AuditOAuthTokenRevoke,
			Resource:   "oauth_token",
			ResourceID: token.TokenID,
			ClientID:   clientID,
			Detail:     "oauth token revoked",
			Status:     "success",
		}).Error
	})
	return nil
}

func (s *OIDCStorage) GetRefreshTokenInfo(ctx context.Context, clientID, token string) (string, string, error) {
	var t model.Token
	if err := s.db.WithContext(ctx).Where("refresh_token = ?", token).First(&t).Error; err != nil {
		return "", "", errors.New("refresh token not found")
	}
	return t.UserID, t.RefreshToken, nil
}
