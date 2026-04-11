package model

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"time"
)

const IntegrationTokenPrefix = "idt_"

const (
	IntegrationTokenScopeAppsWrite     = "apps:write"
	IntegrationTokenScopeAppsRead      = "apps:read"
	IntegrationTokenScopeWebhooksRead  = "webhooks:read"
	IntegrationTokenScopeWebhooksWrite = "webhooks:write"
)

type ServiceAccount struct {
	Base
	Name               string     `gorm:"type:varchar(255)" json:"name"`
	Description        string     `gorm:"type:text" json:"description"`
	WorkspaceType      string     `gorm:"index;type:varchar(20)" json:"workspace_type"`
	OwnerUserID        string     `gorm:"index;type:varchar(36)" json:"owner_user_id,omitempty"`
	OrganizationID     string     `gorm:"index;type:varchar(36)" json:"organization_id,omitempty"`
	ApplicationID      string     `gorm:"index;type:varchar(36)" json:"application_id,omitempty"`
	ApplicationIDs     []string   `gorm:"-" json:"application_ids,omitempty"`
	WebhookEndpointID  string     `gorm:"index;type:varchar(36)" json:"webhook_endpoint_id,omitempty"`
	WebhookEndpointIDs []string   `gorm:"-" json:"webhook_endpoint_ids,omitempty"`
	RateLimitPerMinute int        `gorm:"default:0" json:"rate_limit_per_minute"`
	Disabled           bool       `gorm:"default:false" json:"disabled"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
}

func (ServiceAccount) TableName() string { return "service_accounts" }

type IntegrationToken struct {
	Base
	ServiceAccountID string     `gorm:"index;type:varchar(36)" json:"service_account_id"`
	Name             string     `gorm:"type:varchar(255)" json:"name"`
	TokenPrefix      string     `gorm:"index;type:varchar(32)" json:"token_prefix"`
	TokenHash        string     `gorm:"uniqueIndex;type:varchar(64)" json:"-"`
	Scopes           string     `gorm:"type:varchar(255)" json:"scopes,omitempty"`
	ExpiresAt        *time.Time `gorm:"index" json:"expires_at,omitempty"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
	RevokedAt        *time.Time `gorm:"index" json:"revoked_at,omitempty"`
}

func (IntegrationToken) TableName() string { return "integration_tokens" }

func HashIntegrationToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func SupportedIntegrationTokenScopes() []string {
	return []string{
		IntegrationTokenScopeAppsWrite,
		IntegrationTokenScopeAppsRead,
		IntegrationTokenScopeWebhooksRead,
		IntegrationTokenScopeWebhooksWrite,
	}
}

func LegacyIntegrationTokenScopes() []string {
	return []string{
		IntegrationTokenScopeAppsRead,
		IntegrationTokenScopeWebhooksRead,
	}
}

func JoinIntegrationTokenScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	items := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		items = append(items, scope)
	}
	slices.Sort(items)
	items = slices.Compact(items)
	return strings.Join(items, ",")
}

func SplitIntegrationTokenScopes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	items := strings.Split(raw, ",")
	scopes := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		scopes = append(scopes, item)
	}
	slices.Sort(scopes)
	return slices.Compact(scopes)
}
