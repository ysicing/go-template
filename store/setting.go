package store

import (
	"context"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

// Well-known setting keys.
const (
	SettingRegisterEnabled       = "register_enabled"
	SettingPasswordPolicyEnabled = "password_policy_enabled"
	SettingSiteTitle             = "site_title"
	SettingCORSOrigins           = "cors_origins"
	SettingWebAuthnRPID          = "webauthn_rp_id"
	SettingWebAuthnRPDisplay     = "webauthn_rp_display_name"
	SettingWebAuthnRPOrigins     = "webauthn_rp_origins"

	SettingTurnstileSiteKey   = "turnstile_site_key"
	SettingTurnstileSecretKey = "turnstile_secret_key"

	SettingSMTPHost                 = "smtp_host"
	SettingSMTPPort                 = "smtp_port"
	SettingSMTPUsername             = "smtp_username"
	SettingSMTPPassword             = "smtp_password"
	SettingSMTPFromAddress          = "smtp_from_address"
	SettingSMTPTLS                  = "smtp_tls"
	SettingEmailVerificationEnabled = "email_verification_enabled"
	SettingEmailDomainMode          = "email_domain_mode"      // "whitelist" / "blacklist" / "disabled"
	SettingEmailDomainWhitelist     = "email_domain_whitelist" // 逗号分隔的域名
	SettingEmailDomainBlacklist     = "email_domain_blacklist" // 逗号分隔的域名

	SettingInviteRewardEnabled = "invite_reward_enabled"
	SettingInviteRewardMin     = "invite_reward_min"
	SettingInviteRewardMax     = "invite_reward_max"
)

const (
	settingCacheTTL  = 10 * time.Minute
	settingMissValue = "\x00"
)

// SettingStore handles persistence for system settings.
type SettingStore struct {
	db    *gorm.DB
	cache Cache
}

// NewSettingStore creates a SettingStore.
func NewSettingStore(db *gorm.DB, cache Cache) *SettingStore {
	return &SettingStore{db: db, cache: cache}
}

// Get retrieves a setting value by key. Returns defaultVal if not found.
func (s *SettingStore) Get(key, defaultVal string) string {
	return s.GetWithContext(context.Background(), key, defaultVal)
}

// GetWithContext retrieves a setting value by key with a custom context.
func (s *SettingStore) GetWithContext(ctx context.Context, key, defaultVal string) string {
	cacheKey := "setting:" + key

	// Try cache first.
	if v, err := s.cache.Get(ctx, cacheKey); err == nil {
		if v == settingMissValue { // cached miss
			return defaultVal
		}
		return v
	}

	// Fall back to DB.
	var setting model.Setting
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		// Cache the miss to avoid repeated DB queries.
		_ = s.cache.Set(ctx, cacheKey, settingMissValue, settingCacheTTL)
		return defaultVal
	}

	// Cache the value.
	_ = s.cache.Set(ctx, cacheKey, setting.Value, settingCacheTTL)
	return setting.Value
}

// Set creates or updates a setting.
func (s *SettingStore) Set(ctx context.Context, key, value string) error {
	setting := model.Setting{Key: key, Value: value}
	err := s.db.WithContext(ctx).Where("key = ?", key).Assign(model.Setting{Value: value}).FirstOrCreate(&setting).Error
	if err == nil {
		_ = s.cache.Set(ctx, "setting:"+key, value, settingCacheTTL)
	}
	return err
}

// List returns all settings.
func (s *SettingStore) List(ctx context.Context) ([]model.Setting, error) {
	var settings []model.Setting
	if err := s.db.WithContext(ctx).Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

// Delete removes a setting by key.
func (s *SettingStore) Delete(ctx context.Context, key string) error {
	err := s.db.WithContext(ctx).Where("key = ?", key).Delete(&model.Setting{}).Error
	if err == nil {
		_ = s.cache.Set(ctx, "setting:"+key, settingMissValue, settingCacheTTL)
	}
	return err
}

// GetBool returns a boolean setting. Returns defaultVal if not found.
func (s *SettingStore) GetBool(key string, defaultVal bool) bool {
	v := s.Get(key, "")
	if v == "" {
		return defaultVal
	}
	return v == "true" || v == "1"
}

// GetInt returns an integer setting. Returns defaultVal on missing or invalid values.
func (s *SettingStore) GetInt(key string, defaultVal int) int {
	v := strings.TrimSpace(s.Get(key, ""))
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// GetStringSlice returns a setting value split by comma. Returns defaultVal if not found.
func (s *SettingStore) GetStringSlice(key string, defaultVal []string) []string {
	v := s.Get(key, "")
	if v == "" {
		return defaultVal
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return defaultVal
	}
	return result
}
