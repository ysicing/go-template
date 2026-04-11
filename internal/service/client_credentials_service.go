package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

const defaultClientCredentialsAccessTokenTTL = 5 * time.Minute

var (
	ErrClientCredentialsInvalidClient      = errors.New("invalid client credentials")
	ErrClientCredentialsUnauthorizedClient = errors.New("client is not allowed to use client_credentials")
	ErrClientCredentialsInvalidScope       = errors.New("requested scope is not allowed for client")
	ErrClientCredentialsAuditRequired      = errors.New("audit store is required")
)

type ClientCredentialsServiceDeps struct {
	DB             *gorm.DB
	Clients        *store.OAuthClientStore
	Audit          *store.AuditLogStore
	AccessTokenTTL time.Duration
}

type ClientCredentialsExchangeInput struct {
	ClientID     string
	ClientSecret string
	GrantType    string
	Scope        string
}

type ClientCredentialsTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope,omitempty"`
}

type ClientCredentialsIntrospectionInput struct {
	ClientID     string
	ClientSecret string
	Token        string
}

type ClientCredentialsRevokeInput struct {
	ClientID     string
	ClientSecret string
	Token        string
}

type ClientCredentialsService struct {
	db             *gorm.DB
	clients        *store.OAuthClientStore
	audit          *store.AuditLogStore
	accessTokenTTL time.Duration
}

func NewClientCredentialsService(deps ClientCredentialsServiceDeps) *ClientCredentialsService {
	ttl := deps.AccessTokenTTL
	if ttl <= 0 {
		ttl = defaultClientCredentialsAccessTokenTTL
	}
	return &ClientCredentialsService{
		db:             deps.DB,
		clients:        deps.Clients,
		audit:          deps.Audit,
		accessTokenTTL: ttl,
	}
}

func (s *ClientCredentialsService) Exchange(ctx context.Context, input ClientCredentialsExchangeInput) (*ClientCredentialsTokenResponse, error) {
	if s.audit == nil {
		return nil, ErrClientCredentialsAuditRequired
	}
	client, err := s.authenticateClient(ctx, input.ClientID, input.ClientSecret)
	if err != nil {
		return nil, err
	}
	if !containsValue(parseDelimitedValues(client.GrantTypes), "client_credentials") {
		return nil, ErrClientCredentialsUnauthorizedClient
	}

	scopes, err := resolveClientCredentialsScopes(client.Scopes, input.Scope)
	if err != nil {
		return nil, err
	}

	accessToken := uuid.NewString()
	expiresAt := time.Now().Add(s.accessTokenTTL)
	token := &model.Token{
		TokenID:     accessToken,
		UserID:      "",
		SubjectType: "oauth_client",
		SubjectID:   client.ClientID,
		ClientID:    client.ClientID,
		Scopes:      strings.Join(scopes, ","),
		TokenType:   "access",
		ExpiresAt:   expiresAt,
	}

	auditLog := &model.AuditLog{
		Action:     model.AuditOAuthClientTokenIssue,
		Resource:   "oauth_client",
		ResourceID: accessToken,
		ClientID:   client.ClientID,
		Detail:     "source=system oauth client token issued",
		Status:     "success",
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		return tx.Create(auditLog).Error
	}); err != nil {
		return nil, err
	}

	resp := &ClientCredentialsTokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.accessTokenTTL.Seconds()),
		Scope:       strings.Join(scopes, " "),
	}
	return resp, nil
}

func (s *ClientCredentialsService) Introspect(ctx context.Context, input ClientCredentialsIntrospectionInput) (*oidc.IntrospectionResponse, bool, error) {
	client, err := s.authenticateClient(ctx, input.ClientID, input.ClientSecret)
	if err != nil {
		return nil, false, err
	}
	return s.IntrospectForClient(ctx, client, input.Token)
}

func (s *ClientCredentialsService) IntrospectForClient(ctx context.Context, client *model.OAuthClient, tokenValue string) (*oidc.IntrospectionResponse, bool, error) {
	if client == nil {
		return nil, false, ErrClientCredentialsInvalidClient
	}
	token, handled, err := s.findClientPrincipalToken(ctx, tokenValue)
	if err != nil || !handled {
		return nil, handled, err
	}

	resp := &oidc.IntrospectionResponse{}
	if token.ClientID != client.ClientID || token.Revoked || time.Now().After(token.ExpiresAt) {
		return resp, true, nil
	}

	resp.Active = true
	resp.Subject = token.SubjectID
	resp.ClientID = token.ClientID
	resp.Scope = oidc.SpaceDelimitedArray(parseDelimitedValues(token.Scopes))
	resp.TokenType = token.TokenType
	resp.Expiration = oidc.FromTime(token.ExpiresAt)
	return resp, true, nil
}

func (s *ClientCredentialsService) Revoke(ctx context.Context, input ClientCredentialsRevokeInput) (bool, error) {
	client, err := s.authenticateClient(ctx, input.ClientID, input.ClientSecret)
	if err != nil {
		return false, err
	}
	return s.RevokeForClient(ctx, client, input.Token)
}

func (s *ClientCredentialsService) RevokeForClient(ctx context.Context, client *model.OAuthClient, tokenValue string) (bool, error) {
	if client == nil {
		return false, ErrClientCredentialsInvalidClient
	}
	token, handled, err := s.findClientPrincipalToken(ctx, tokenValue)
	if err != nil || !handled {
		return handled, err
	}
	if token.ClientID != client.ClientID {
		return true, nil
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Delete(&model.Token{}, "id = ?", token.ID).Error; err != nil {
			return err
		}
		return tx.Create(&model.AuditLog{
			Action:     model.AuditOAuthTokenRevoke,
			Resource:   "oauth_token",
			ResourceID: token.TokenID,
			ClientID:   client.ClientID,
			Detail:     "oauth token revoked",
			Status:     "success",
		}).Error
	}); err != nil {
		return true, err
	}

	return true, nil
}

func resolveClientCredentialsScopes(clientScopesRaw, requestedScopesRaw string) ([]string, error) {
	allowed := parseDelimitedValues(clientScopesRaw)
	if requestedScopesRaw == "" {
		return allowed, nil
	}

	requested := parseDelimitedValues(requestedScopesRaw)
	for _, scope := range requested {
		if !containsValue(allowed, scope) {
			return nil, ErrClientCredentialsInvalidScope
		}
	}
	return requested, nil
}

func parseDelimitedValues(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", " ")
	values := strings.Fields(raw)
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func containsValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *ClientCredentialsService) authenticateClient(ctx context.Context, clientID, clientSecret string) (*model.OAuthClient, error) {
	client, err := s.clients.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, ErrClientCredentialsInvalidClient
	}
	if !client.CheckSecret(clientSecret) {
		return nil, ErrClientCredentialsInvalidClient
	}
	return client, nil
}

func (s *ClientCredentialsService) findClientPrincipalToken(ctx context.Context, tokenValue string) (*model.Token, bool, error) {
	var token model.Token
	if err := s.db.WithContext(ctx).Where("token_id = ?", tokenValue).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if token.SubjectType != "oauth_client" {
		return nil, false, nil
	}
	return &token, true, nil
}
