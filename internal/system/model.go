package system

import "time"

type BootstrapState struct {
	ID            uint      `gorm:"primaryKey"`
	InitializedAt time.Time `gorm:"not null"`
	Version       string    `gorm:"size:32;not null"`
}

type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Group     string    `gorm:"size:64;index;not null" json:"group"`
	Key       string    `gorm:"size:128;uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func DefaultSettings() []Setting {
	return []Setting{
		{Group: "site", Key: "site.name", Value: "Go Template"},
		{Group: "site", Key: "site.allow_register", Value: "false"},
		{Group: "auth", Key: "auth.access_ttl", Value: "15m"},
		{Group: "auth", Key: "auth.refresh_ttl", Value: "168h"},
	}
}

