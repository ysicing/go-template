package clientcredentialsservice

import (
	"context"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/stretchr/testify/require"
)

func createClientCredentialsTestClient(t *testing.T, clients *store.OAuthClientStore, grantTypes, scopes string) (*model.OAuthClient, string) {
	t.Helper()

	client := &model.OAuthClient{
		Name:         "Machine Client",
		ClientID:     "client-1",
		GrantTypes:   grantTypes,
		Scopes:       scopes,
		RedirectURIs: "https://example.com/callback",
	}
	secret := "super-secret"
	require.NoError(t, client.SetSecret(secret))
	require.NoError(t, clients.Create(context.Background(), client))
	return client, secret
}

func TestClientCredentialsServiceExchangeIssuesAccessToken(t *testing.T) {
	db := setupOrganizationServiceTestDB(t)
	clients := store.NewOAuthClientStore(db)
	audit := store.NewAuditLogStore(db)
	client, secret := createClientCredentialsTestClient(t, clients, "client_credentials", "openid profile")

	svc := NewClientCredentialsService(ClientCredentialsServiceDeps{
		Clients: clients,
		Audit:   audit,
	})

	resp, err := svc.Exchange(context.Background(), ClientCredentialsExchangeInput{
		ClientID:     client.ClientID,
		ClientSecret: secret,
		GrantType:    "client_credentials",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.AccessToken)
	require.Equal(t, "Bearer", resp.TokenType)
	require.Equal(t, int64((5 * time.Minute).Seconds()), resp.ExpiresIn)
	require.Equal(t, "openid profile", resp.Scope)

	var token model.Token
	require.NoError(t, db.WithContext(context.Background()).Where("token_id = ?", resp.AccessToken).First(&token).Error)
	require.Equal(t, "oauth_client", token.SubjectType)
	require.Equal(t, client.ClientID, token.SubjectID)
	require.Empty(t, token.UserID)
	require.Equal(t, client.ClientID, token.ClientID)
	require.Equal(t, "access", token.TokenType)

	var logs []model.AuditLog
	require.NoError(t, db.WithContext(context.Background()).Where("action = ?", model.AuditOAuthClientTokenIssue).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, client.ClientID, logs[0].ClientID)
	require.Equal(t, "oauth_client", logs[0].Resource)
	require.Equal(t, "success", logs[0].Status)
}

func TestClientCredentialsServiceExchangeRejectsInvalidScopeSubset(t *testing.T) {
	db := setupOrganizationServiceTestDB(t)
	clients := store.NewOAuthClientStore(db)
	client, secret := createClientCredentialsTestClient(t, clients, "client_credentials", "openid profile")

	svc := NewClientCredentialsService(ClientCredentialsServiceDeps{
		Clients: clients,
		Audit:   store.NewAuditLogStore(db),
	})

	_, err := svc.Exchange(context.Background(), ClientCredentialsExchangeInput{
		ClientID:     client.ClientID,
		ClientSecret: secret,
		GrantType:    "client_credentials",
		Scope:        "openid admin",
	})
	require.ErrorIs(t, err, ErrClientCredentialsInvalidScope)
}

func TestClientCredentialsServiceExchangeRejectsUnauthorizedGrantType(t *testing.T) {
	db := setupOrganizationServiceTestDB(t)
	clients := store.NewOAuthClientStore(db)
	client, secret := createClientCredentialsTestClient(t, clients, "authorization_code", "openid profile")

	svc := NewClientCredentialsService(ClientCredentialsServiceDeps{
		Clients: clients,
		Audit:   store.NewAuditLogStore(db),
	})

	_, err := svc.Exchange(context.Background(), ClientCredentialsExchangeInput{
		ClientID:     client.ClientID,
		ClientSecret: secret,
		GrantType:    "client_credentials",
	})
	require.ErrorIs(t, err, ErrClientCredentialsUnauthorizedClient)
}

func TestClientCredentialsServiceExchangeRejectsWrongSecret(t *testing.T) {
	db := setupOrganizationServiceTestDB(t)
	clients := store.NewOAuthClientStore(db)
	client, _ := createClientCredentialsTestClient(t, clients, "client_credentials", "openid profile")

	svc := NewClientCredentialsService(ClientCredentialsServiceDeps{
		Clients: clients,
		Audit:   store.NewAuditLogStore(db),
	})

	_, err := svc.Exchange(context.Background(), ClientCredentialsExchangeInput{
		ClientID:     client.ClientID,
		ClientSecret: "wrong-secret",
		GrantType:    "client_credentials",
	})
	require.ErrorIs(t, err, ErrClientCredentialsInvalidClient)
}

func TestClientCredentialsServiceIntrospectReturnsActiveForOwnedToken(t *testing.T) {
	db := setupOrganizationServiceTestDB(t)
	clients := store.NewOAuthClientStore(db)
	client, secret := createClientCredentialsTestClient(t, clients, "client_credentials", "openid profile")

	token := &model.Token{
		TokenID:     "client-token-introspect",
		SubjectType: "oauth_client",
		SubjectID:   client.ClientID,
		ClientID:    client.ClientID,
		Scopes:      "openid profile",
		TokenType:   "access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	require.NoError(t, db.WithContext(context.Background()).Create(token).Error)

	svc := NewClientCredentialsService(ClientCredentialsServiceDeps{
		Clients: clients,
		Audit:   store.NewAuditLogStore(db),
	})

	resp, handled, err := svc.Introspect(context.Background(), ClientCredentialsIntrospectionInput{
		ClientID:     client.ClientID,
		ClientSecret: secret,
		Token:        token.TokenID,
	})
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, resp.Active)
	require.Equal(t, client.ClientID, resp.Subject)
	require.Equal(t, client.ClientID, resp.ClientID)
	require.Equal(t, "access", resp.TokenType)
	require.Equal(t, []string{"openid", "profile"}, []string(resp.Scope))
}

func TestClientCredentialsServiceRevokeDeletesOwnedTokenAndWritesAudit(t *testing.T) {
	db := setupOrganizationServiceTestDB(t)
	clients := store.NewOAuthClientStore(db)
	client, secret := createClientCredentialsTestClient(t, clients, "client_credentials", "openid profile")

	token := &model.Token{
		TokenID:     "client-token-revoke",
		SubjectType: "oauth_client",
		SubjectID:   client.ClientID,
		ClientID:    client.ClientID,
		Scopes:      "openid profile",
		TokenType:   "access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	require.NoError(t, db.WithContext(context.Background()).Create(token).Error)

	svc := NewClientCredentialsService(ClientCredentialsServiceDeps{
		Clients: clients,
		Audit:   store.NewAuditLogStore(db),
	})

	handled, err := svc.Revoke(context.Background(), ClientCredentialsRevokeInput{
		ClientID:     client.ClientID,
		ClientSecret: secret,
		Token:        token.TokenID,
	})
	require.NoError(t, err)
	require.True(t, handled)

	var count int64
	require.NoError(t, db.WithContext(context.Background()).Model(&model.Token{}).Where("token_id = ?", token.TokenID).Count(&count).Error)
	require.Zero(t, count)

	var logs []model.AuditLog
	require.NoError(t, db.WithContext(context.Background()).Where("action = ?", model.AuditOAuthTokenRevoke).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, client.ClientID, logs[0].ClientID)
}

func TestClientCredentialsServiceOperatorIntrospectReturnsActiveForOwnedToken(t *testing.T) {
	db := setupOrganizationServiceTestDB(t)
	clients := store.NewOAuthClientStore(db)
	client, _ := createClientCredentialsTestClient(t, clients, "client_credentials", "openid profile")

	token := &model.Token{
		TokenID:     "operator-token-introspect",
		SubjectType: "oauth_client",
		SubjectID:   client.ClientID,
		ClientID:    client.ClientID,
		Scopes:      "openid profile",
		TokenType:   "access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	require.NoError(t, db.WithContext(context.Background()).Create(token).Error)

	svc := NewClientCredentialsService(ClientCredentialsServiceDeps{
		Clients: clients,
		Audit:   store.NewAuditLogStore(db),
	})

	resp, handled, err := svc.IntrospectForClient(context.Background(), client, token.TokenID)
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, resp.Active)
	require.Equal(t, client.ClientID, resp.ClientID)
	require.Equal(t, client.ClientID, resp.Subject)
}

func TestClientCredentialsServiceOperatorRevokeDeletesOwnedTokenAndWritesAudit(t *testing.T) {
	db := setupOrganizationServiceTestDB(t)
	clients := store.NewOAuthClientStore(db)
	client, _ := createClientCredentialsTestClient(t, clients, "client_credentials", "openid profile")

	token := &model.Token{
		TokenID:     "operator-token-revoke",
		SubjectType: "oauth_client",
		SubjectID:   client.ClientID,
		ClientID:    client.ClientID,
		Scopes:      "openid profile",
		TokenType:   "access",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	require.NoError(t, db.WithContext(context.Background()).Create(token).Error)

	svc := NewClientCredentialsService(ClientCredentialsServiceDeps{
		Clients: clients,
		Audit:   store.NewAuditLogStore(db),
	})

	handled, err := svc.RevokeForClient(context.Background(), client, token.TokenID)
	require.NoError(t, err)
	require.True(t, handled)

	var count int64
	require.NoError(t, db.WithContext(context.Background()).Model(&model.Token{}).Where("token_id = ?", token.TokenID).Count(&count).Error)
	require.Zero(t, count)

	var logs []model.AuditLog
	require.NoError(t, db.WithContext(context.Background()).Where("action = ?", model.AuditOAuthTokenRevoke).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, client.ClientID, logs[0].ClientID)
}
