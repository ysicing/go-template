package oidcstore

import (
	"context"
	"crypto/rsa"
	"sync"
	"time"

	"github.com/ysicing/go-template/model"
	rootstore "github.com/ysicing/go-template/store"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

type oidcAuthRequest struct {
	id            string
	creationDate  time.Time
	clientID      string
	redirectURI   string
	scopes        []string
	prompt        []string
	responseType  oidc.ResponseType
	responseMode  oidc.ResponseMode
	nonce         string
	state         string
	codeChallenge *oidc.CodeChallenge
	userID        string
	done          bool
	authTime      time.Time
	loginURL      string
}

func (a *oidcAuthRequest) GetID() string                         { return a.id }
func (a *oidcAuthRequest) GetACR() string                        { return "" }
func (a *oidcAuthRequest) GetAMR() []string                      { return nil }
func (a *oidcAuthRequest) GetAudience() []string                 { return []string{a.clientID} }
func (a *oidcAuthRequest) GetAuthTime() time.Time                { return a.authTime }
func (a *oidcAuthRequest) GetClientID() string                   { return a.clientID }
func (a *oidcAuthRequest) GetCodeChallenge() *oidc.CodeChallenge { return a.codeChallenge }
func (a *oidcAuthRequest) GetNonce() string                      { return a.nonce }
func (a *oidcAuthRequest) GetRedirectURI() string                { return a.redirectURI }
func (a *oidcAuthRequest) GetResponseType() oidc.ResponseType    { return a.responseType }
func (a *oidcAuthRequest) GetResponseMode() oidc.ResponseMode    { return a.responseMode }
func (a *oidcAuthRequest) GetScopes() []string                   { return a.scopes }
func (a *oidcAuthRequest) GetState() string                      { return a.state }
func (a *oidcAuthRequest) GetSubject() string                    { return a.userID }
func (a *oidcAuthRequest) Done() bool                            { return a.done }

type oidcClient struct {
	client     *model.OAuthClient
	loginURL   string
	idTokenTTL time.Duration
}

func (c *oidcClient) GetID() string { return c.client.ClientID }
func (c *oidcClient) RedirectURIs() []string {
	return rootstore.SplitTrimmed(c.client.RedirectURIs)
}
func (c *oidcClient) PostLogoutRedirectURIs() []string { return nil }
func (c *oidcClient) ApplicationType() op.ApplicationType {
	return op.ApplicationTypeWeb
}
func (c *oidcClient) AuthMethod() oidc.AuthMethod { return oidc.AuthMethodBasic }
func (c *oidcClient) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{oidc.ResponseTypeCode}
}
func (c *oidcClient) GrantTypes() []oidc.GrantType {
	raw := rootstore.SplitTrimmed(c.client.GrantTypes)
	types := make([]oidc.GrantType, 0, len(raw))
	for _, g := range raw {
		types = append(types, oidc.GrantType(g))
	}
	if len(types) == 0 {
		types = append(types, oidc.GrantTypeCode)
	}
	return types
}
func (c *oidcClient) LoginURL(id string) string           { return c.loginURL + "?id=" + id }
func (c *oidcClient) AccessTokenType() op.AccessTokenType { return op.AccessTokenTypeBearer }
func (c *oidcClient) IDTokenLifetime() time.Duration      { return c.idTokenTTL }
func (c *oidcClient) DevMode() bool                       { return false }
func (c *oidcClient) RestrictAdditionalIdTokenScopes() func([]string) []string {
	return nil
}
func (c *oidcClient) RestrictAdditionalAccessTokenScopes() func([]string) []string {
	return nil
}
func (c *oidcClient) IsScopeAllowed(scope string) bool {
	for _, s := range rootstore.SplitTrimmed(c.client.Scopes) {
		if s == scope {
			return true
		}
	}
	return false
}
func (c *oidcClient) IDTokenUserinfoClaimsAssertion() bool { return false }
func (c *oidcClient) ClockSkew() time.Duration             { return 0 }

type oidcRefreshToken struct {
	id       string
	token    string
	userID   string
	clientID string
	scopes   []string
	authTime time.Time
	expiry   time.Time
}

func (r *oidcRefreshToken) GetAMR() []string                 { return nil }
func (r *oidcRefreshToken) GetAudience() []string            { return []string{r.clientID} }
func (r *oidcRefreshToken) GetAuthTime() time.Time           { return r.authTime }
func (r *oidcRefreshToken) GetClientID() string              { return r.clientID }
func (r *oidcRefreshToken) GetScopes() []string              { return r.scopes }
func (r *oidcRefreshToken) GetSubject() string               { return r.userID }
func (r *oidcRefreshToken) SetCurrentScopes(scopes []string) { r.scopes = scopes }

type signingKeyData struct {
	id         string
	algorithm  jose.SignatureAlgorithm
	privateKey *rsa.PrivateKey
}

func (s *signingKeyData) SignatureAlgorithm() jose.SignatureAlgorithm { return s.algorithm }
func (s *signingKeyData) Key() any                                    { return s.privateKey }
func (s *signingKeyData) ID() string                                  { return s.id }

type publicKeyData struct {
	id        string
	algorithm jose.SignatureAlgorithm
	key       *rsa.PublicKey
}

func (p *publicKeyData) ID() string                         { return p.id }
func (p *publicKeyData) Algorithm() jose.SignatureAlgorithm { return p.algorithm }
func (p *publicKeyData) Use() string                        { return "sig" }
func (p *publicKeyData) Key() any                           { return p.key }

const (
	oidcSigningKeyRotationInterval = 30 * 24 * time.Hour
	oidcSigningKeyRetainDuration   = 90 * 24 * time.Hour
)

type OIDCStorage struct {
	db              *gorm.DB
	cache           rootstore.Cache
	signingKey      signingKeyData
	encPassphrase   string
	users           *rootstore.UserStore
	clients         *rootstore.OAuthClientStore
	loginURL        string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	authRequestTTL  time.Duration

	signingMu sync.RWMutex
}

// NewOIDCStorage creates a new DB-backed OIDCStorage.
func NewOIDCStorage(ctx context.Context, db *gorm.DB, cache rootstore.Cache, users *rootstore.UserStore, clients *rootstore.OAuthClientStore, loginURL string, encryptionKey string, accessTTL, refreshTTL, authReqTTL time.Duration) (*OIDCStorage, error) {
	if accessTTL == 0 {
		accessTTL = 5 * time.Minute
	}
	if refreshTTL == 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	if authReqTTL == 0 {
		authReqTTL = 10 * time.Minute
	}
	s := &OIDCStorage{
		db:              db,
		cache:           cache,
		users:           users,
		clients:         clients,
		loginURL:        loginURL,
		encPassphrase:   encryptionKey,
		accessTokenTTL:  accessTTL,
		refreshTokenTTL: refreshTTL,
		authRequestTTL:  authReqTTL,
	}
	if err := s.loadOrCreateSigningKey(ctx); err != nil {
		return nil, err
	}
	go s.cleanupLoop(ctx)
	return s, nil
}
