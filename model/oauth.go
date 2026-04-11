package model

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// OAuthClient represents a registered OAuth2 client application.
type OAuthClient struct {
	Base
	Name           string `gorm:"type:varchar(255)" json:"name"`
	ClientID       string `gorm:"uniqueIndex;type:varchar(255)" json:"client_id"`
	ClientSecret   string `gorm:"type:varchar(255)" json:"-"`
	RedirectURIs   string `gorm:"type:text" json:"redirect_uris"`
	GrantTypes     string `gorm:"type:varchar(255);default:'authorization_code'" json:"grant_types"`
	Scopes         string `gorm:"type:varchar(512);default:'openid profile email'" json:"scopes"`
	RequireConsent bool   `gorm:"default:false" json:"require_consent"`
	UserID         string `gorm:"type:varchar(36);index" json:"user_id"`
	OrganizationID string `gorm:"type:varchar(36);index" json:"organization_id,omitempty"`
}

func (OAuthClient) TableName() string { return "oauth_clients" }

// SetSecret hashes and stores the client secret.
func (c *OAuthClient) SetSecret(secret string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	c.ClientSecret = string(hash)
	return nil
}

// CheckSecret verifies a client secret against the stored hash.
func (c *OAuthClient) CheckSecret(secret string) bool {
	return bcrypt.CompareHashAndPassword([]byte(c.ClientSecret), []byte(secret)) == nil
}

// AuthRequest tracks an in-flight OAuth2 authorization request.
type AuthRequest struct {
	Base
	ClientID            string    `gorm:"type:varchar(255);index" json:"client_id"`
	RedirectURI         string    `gorm:"type:varchar(512)" json:"redirect_uri"`
	Scopes              string    `gorm:"type:varchar(512)" json:"scopes"`
	Prompt              string    `gorm:"type:varchar(128)" json:"prompt"`
	State               string    `gorm:"type:varchar(255)" json:"state"`
	Nonce               string    `gorm:"type:varchar(255)" json:"nonce"`
	ResponseType        string    `gorm:"type:varchar(50)" json:"response_type"`
	ResponseMode        string    `gorm:"type:varchar(50)" json:"response_mode"`
	CodeChallenge       string    `gorm:"type:varchar(255)" json:"code_challenge"`
	CodeChallengeMethod string    `gorm:"type:varchar(10)" json:"code_challenge_method"`
	UserID              string    `gorm:"type:varchar(36);index" json:"user_id"`
	Code                string    `gorm:"type:varchar(255);index" json:"code"`
	Done                bool      `gorm:"default:false" json:"done"`
	AuthTime            time.Time `json:"auth_time"`
	ExpiresAt           time.Time `json:"expires_at"`
}

func (AuthRequest) TableName() string { return "auth_requests" }

// Token represents a persisted access or refresh token.
type Token struct {
	Base
	TokenID      string    `gorm:"uniqueIndex;type:varchar(36)" json:"token_id"`
	UserID       string    `gorm:"type:varchar(36);index" json:"user_id"`
	SubjectType  string    `gorm:"type:varchar(32);index" json:"subject_type"`
	SubjectID    string    `gorm:"type:varchar(255);index" json:"subject_id"`
	ClientID     string    `gorm:"type:varchar(255);index" json:"client_id"`
	Scopes       string    `gorm:"type:varchar(512)" json:"scopes"`
	TokenType    string    `gorm:"type:varchar(20);default:'access'" json:"token_type"`
	RefreshToken string    `gorm:"type:varchar(512);index" json:"-"`
	ExpiresAt    time.Time `json:"expires_at"`
	Revoked      bool      `gorm:"default:false" json:"revoked"`
}

func (Token) TableName() string { return "tokens" }

type OAuthConsentGrant struct {
	Base
	UserID   string `gorm:"uniqueIndex:idx_oauth_consent_grants_user_client;type:varchar(36)" json:"user_id"`
	ClientID string `gorm:"uniqueIndex:idx_oauth_consent_grants_user_client;type:varchar(255)" json:"client_id"`
	Scopes   string `gorm:"type:varchar(512)" json:"scopes"`
}

func (OAuthConsentGrant) TableName() string { return "oauth_consent_grants" }

// SigningKey stores the OIDC provider's signing key material.
type SigningKey struct {
	ID         string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Algorithm  string    `gorm:"type:varchar(10)" json:"algorithm"`
	PrivateKey string    `gorm:"type:text" json:"-"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func (SigningKey) TableName() string { return "signing_keys" }

// SocialProvider stores OAuth2 social login provider configuration.
type SocialProvider struct {
	Base
	Name         string `gorm:"uniqueIndex;type:varchar(50)" json:"name"`
	ClientID     string `gorm:"type:varchar(255)" json:"client_id"`
	ClientSecret string `gorm:"type:varchar(512)" json:"-"`
	RedirectURL  string `gorm:"type:varchar(512)" json:"redirect_url"`
	Enabled      bool   `gorm:"type:bool;default:true" json:"enabled"`
}

func (SocialProvider) TableName() string { return "social_providers" }

// Setting stores a system configuration key-value pair.
type Setting struct {
	Key   string `gorm:"primaryKey;type:varchar(100)" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

func (Setting) TableName() string { return "settings" }

// APIRefreshToken stores server-side refresh tokens for the local JWT auth system.
type APIRefreshToken struct {
	Base
	UserID     string    `gorm:"index;type:varchar(36)" json:"user_id"`
	TokenHash  string    `gorm:"uniqueIndex;type:varchar(64)" json:"-"`
	Family     string    `gorm:"index;type:varchar(36)" json:"-"`
	ExpiresAt  time.Time `json:"expires_at"`
	IP         string    `gorm:"type:varchar(45)" json:"ip"`
	UserAgent  string    `gorm:"type:varchar(512)" json:"user_agent"`
	LastUsedAt time.Time `json:"last_used_at"`
}

func (APIRefreshToken) TableName() string { return "api_refresh_tokens" }
