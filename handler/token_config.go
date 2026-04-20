package handler

import (
	"time"

	"github.com/ysicing/go-template/internal/service"
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
func (tc TokenConfig) ToServiceConfig() service.TokenConfig {
	return service.TokenConfig(tc)
}
