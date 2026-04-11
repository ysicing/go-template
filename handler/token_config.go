package handler

import "time"

// TokenConfig aggregates JWT token configuration parameters.
type TokenConfig struct {
	Secret        string
	Issuer        string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	RememberMeTTL time.Duration
}
