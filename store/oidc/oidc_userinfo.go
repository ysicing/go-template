package oidcstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ysicing/go-template/model"
	rootstore "github.com/ysicing/go-template/store"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

func (s *OIDCStorage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	client, err := s.clients.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("client not found: %w", err)
	}
	return &oidcClient{client: client, loginURL: s.loginURL, idTokenTTL: s.accessTokenTTL}, nil
}

func (s *OIDCStorage) AuthorizeClientIDSecret(ctx context.Context, clientID, clientSecret string) error {
	client, err := s.clients.GetByClientID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("client not found: %w", err)
	}
	if !client.CheckSecret(clientSecret) {
		return errors.New("invalid client secret")
	}
	return nil
}

func (s *OIDCStorage) SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID, _ string, scopes []string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	populateUserinfo(userinfo, user, scopes)
	return nil
}

func (s *OIDCStorage) SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID, subject, _ string) error {
	var t model.Token
	if err := s.db.WithContext(ctx).Where("token_id = ?", tokenID).First(&t).Error; err != nil {
		return errors.New("token not found")
	}
	if !isUserSubjectToken(&t) {
		return errors.New("token subject is not a user")
	}
	user, err := s.users.GetByID(ctx, persistedSubjectID(&t, subject))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	populateUserinfo(userinfo, user, rootstore.SplitTrimmed(t.Scopes))
	return nil
}

func (s *OIDCStorage) SetIntrospectionFromToken(ctx context.Context, resp *oidc.IntrospectionResponse, tokenID, subject, _ string) error {
	var t model.Token
	if err := s.db.WithContext(ctx).Where("token_id = ?", tokenID).First(&t).Error; err != nil {
		return nil
	}
	if time.Now().After(t.ExpiresAt) || t.Revoked {
		return nil
	}

	resp.Active = true
	resp.Subject = persistedSubjectID(&t, subject)
	resp.ClientID = t.ClientID
	resp.Scope = oidc.SpaceDelimitedArray(rootstore.SplitTrimmed(t.Scopes))
	resp.TokenType = t.TokenType
	if !isUserSubjectToken(&t) {
		return nil
	}

	user, err := s.users.GetByID(ctx, persistedSubjectID(&t, subject))
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	resp.Subject = user.ID
	resp.Username = user.Username
	return nil
}

func (s *OIDCStorage) GetPrivateClaimsFromScopes(ctx context.Context, userID, _ string, scopes []string) (map[string]any, error) {
	if s.users == nil {
		return nil, nil
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, nil
	}
	claims := make(map[string]any)
	for _, scope := range scopes {
		if scope == "admin" && user.IsAdmin {
			claims["is_admin"] = true
		}
	}
	return claims, nil
}

func (s *OIDCStorage) GetKeyByIDAndClientID(_ context.Context, keyID, _ string) (*jose.JSONWebKey, error) {
	return nil, errors.New("not implemented")
}

func (s *OIDCStorage) ValidateJWTProfileScopes(_ context.Context, _ string, scopes []string) ([]string, error) {
	return scopes, nil
}

func (s *OIDCStorage) Health(_ context.Context) error { return nil }

// FindUserByAccessToken looks up the user associated with an OIDC access token.
func (s *OIDCStorage) FindUserByAccessToken(ctx context.Context, tokenID string) (*model.User, error) {
	var t model.Token
	if err := s.db.WithContext(ctx).
		Where("token_id = ? AND expires_at > ?", tokenID, time.Now()).
		Where("(subject_type = ? OR subject_type = ? OR subject_type IS NULL)", "", "user").
		First(&t).Error; err != nil {
		return nil, errors.New("token not found")
	}
	if t.UserID == "" {
		return nil, errors.New("token subject is not a user")
	}
	return s.users.GetByID(ctx, t.UserID)
}

func populateUserinfo(info *oidc.UserInfo, user *model.User, scopes []string) {
	info.Subject = user.ID
	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			info.Subject = user.ID
		case oidc.ScopeEmail:
			info.Email = user.Email
			info.EmailVerified = oidc.Bool(user.EmailVerified)
		case oidc.ScopeProfile:
			info.PreferredUsername = user.Username
			info.Name = user.Username
			if user.AvatarURL != "" {
				info.Picture = user.AvatarURL
			}
		}
	}
}

func isUserSubjectToken(token *model.Token) bool {
	return token.SubjectType == "" || token.SubjectType == "user"
}

func persistedSubjectID(token *model.Token, fallback string) string {
	if token.SubjectID != "" {
		return token.SubjectID
	}
	if token.UserID != "" {
		return token.UserID
	}
	return fallback
}
