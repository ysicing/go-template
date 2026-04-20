package handler

import (
	"context"

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
