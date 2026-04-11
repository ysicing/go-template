package handler

import (
	"context"
	"time"

	"github.com/ysicing/go-template/model"
)

// settingReader is the minimal read interface for application settings.
// Implemented by *store.SettingStore. Shared across auth, user, and related handlers.
type settingReader interface {
	Get(key, defaultVal string) string
	GetBool(key string, defaultVal bool) bool
	GetInt(key string, defaultVal int) int
	GetStringSlice(key string, defaultVal []string) []string
}

// mfaReader is the minimal read interface for MFA configuration.
// Implemented by *store.MFAStore. Shared across auth and oauth handlers.
type mfaReader interface {
	GetByUserID(ctx context.Context, userID string) (*model.MFAConfig, error)
}

// refreshTokenCreator is the minimal interface for creating API refresh tokens.
// Implemented by *store.APIRefreshTokenStore. Shared across mfa and oauth handlers.
type refreshTokenCreator interface {
	Create(ctx context.Context, rt *model.APIRefreshToken) error
}

// pointManager is the minimal interface needed for quote quota and spending flows.
type pointManager interface {
	GetOrCreateUserPoints(ctx context.Context, userID string) (*model.UserPoints, error)
	SpendPoints(ctx context.Context, userID string, amount int64, kind, reason string) error
}

type cacheStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
	DelIfValue(ctx context.Context, key, value string) (bool, error)
}
