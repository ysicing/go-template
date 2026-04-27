package handler

import (
	"time"

	sessionservice "github.com/ysicing/go-template/internal/service/session"
)

// TokenConfig aggregates JWT token configuration parameters.
type TokenConfig struct {
	Secret        string
	Issuer        string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	RememberMeTTL time.Duration
}

// ToServiceConfig converts to the equivalent service-layer config.
func (tc TokenConfig) ToServiceConfig() sessionservice.TokenConfig {
	return sessionservice.TokenConfig(tc)
}
