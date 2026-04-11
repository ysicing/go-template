package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	QuoteStatusPending  = "pending"
	QuoteStatusApproved = "approved"
	QuoteStatusRejected = "rejected"
)

const (
	QuoteChannelAdmin     = "admin"
	QuoteChannelAPI       = "api"
	QuoteChannelTG        = "tg"
	QuoteChannelAnonymous = "anonymous"
)

const (
	QuoteEntrypointWeb = "web"
	QuoteEntrypointAPI = "api"
	QuoteEntrypointCLI = "cli"
	QuoteEntrypointBot = "bot"
	QuoteEntrypointTG  = "tg"
)

// QuoteTags stores quote tags as JSON.
type QuoteTags []string

func (t QuoteTags) Value() (driver.Value, error) {
	if len(t) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal([]string(t))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (t *QuoteTags) Scan(src any) error {
	if src == nil {
		*t = nil
		return nil
	}
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, t)
	case string:
		return json.Unmarshal([]byte(v), t)
	default:
		return fmt.Errorf("unsupported QuoteTags scan type %T", src)
	}
}

func NormalizeQuoteTags(tags []string) QuoteTags {
	if len(tags) == 0 {
		return QuoteTags{}
	}
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		cleaned := strings.TrimSpace(tag)
		if cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cleaned)
	}
	return QuoteTags(result)
}

type Quote struct {
	Base
	Quote           string     `gorm:"type:varchar(500);not null" json:"quote"`
	Author          string     `gorm:"type:varchar(120);not null;index" json:"author"`
	Source          string     `gorm:"type:varchar(200)" json:"source"`
	Tags            QuoteTags  `gorm:"type:text" json:"tags"`
	Language        string     `gorm:"type:varchar(20);not null;index" json:"language"`
	SubmitterUserID string     `gorm:"type:varchar(36);index" json:"submitter_user_id,omitempty"`
	Status          string     `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	ReviewedBy      string     `gorm:"type:varchar(36)" json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	ReviewNote      string     `gorm:"type:varchar(500)" json:"review_note,omitempty"`
}

func (Quote) TableName() string { return "quotes" }

type QuoteSubmission struct {
	Base
	QuoteID         string `gorm:"type:varchar(36);not null;uniqueIndex" json:"quote_id"`
	SubmitterUserID string `gorm:"type:varchar(36);index" json:"submitter_user_id,omitempty"`
	Channel         string `gorm:"type:varchar(20);not null;index" json:"channel"`
	Entrypoint      string `gorm:"type:varchar(20);not null" json:"entrypoint"`
	IP              string `gorm:"type:varchar(64);index" json:"ip"`
	UserAgent       string `gorm:"type:varchar(500)" json:"user_agent"`
	System          string `gorm:"type:varchar(100)" json:"system"`
	MetadataJSON    string `gorm:"type:text" json:"metadata_json,omitempty"`
}

func (QuoteSubmission) TableName() string { return "quote_submissions" }
