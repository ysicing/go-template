package model

import "time"

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
