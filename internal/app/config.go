package app

import (
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Redis      RedisConfig      `yaml:"redis"`
	JWT        JWTConfig        `yaml:"jwt"`
	Admin      AdminSeedConfig  `yaml:"admin"`
	Security   SecurityConfig   `yaml:"security"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Log        LogConfig        `yaml:"log"`
}

type MonitoringConfig struct {
	AgentLatestVersion          string `yaml:"agent_latest_version"`
	AgentInstallCommandTemplate string `yaml:"agent_install_command_template"`
}

type LogConfig struct {
	Level  string        `yaml:"level"`
	Format string        `yaml:"format"`
	File   LogFileConfig `yaml:"file"`
}

type LogFileConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Path       string `yaml:"path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type ServerConfig struct {
	Addr           string   `yaml:"addr"`
	TrustedProxies []string `yaml:"trusted_proxies"` // CIDR list of trusted proxy IPs (empty = trust none)
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type JWTConfig struct {
	Secret          string        `yaml:"secret"`
	Issuer          string        `yaml:"issuer"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
	RememberMeTTL   time.Duration `yaml:"remember_me_ttl"`
}

type SecurityConfig struct {
	AllowInsecure bool   `yaml:"allow_insecure"`
	EncryptionKey string `yaml:"encryption_key"`
	OIDCSecret    string `yaml:"oidc_secret"`
	Mode          string `yaml:"mode"` // "demo" allows default secrets and relaxed policies; any other value enforces production security
}

type AdminSeedConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Email    string `yaml:"email"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr: ":3206",
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "go-template.db",
		},
		JWT: JWTConfig{
			Secret:          "change-me-in-production",
			Issuer:          "go-template",
			AccessTokenTTL:  15 * time.Minute,    // Access token valid for 15 minutes
			RefreshTokenTTL: 7 * 24 * time.Hour,  // Refresh token valid for 7 days
			RememberMeTTL:   30 * 24 * time.Hour, // Remember me: 30 days
		},
		Security: SecurityConfig{
			AllowInsecure: false,
		},
		Log: LogConfig{
			Level:  "info", // WARNING: "info" level logs full SQL queries which may contain sensitive data. Use "warn" or "error" in production.
			Format: "json",
			File: LogFileConfig{
				MaxSizeMB:  100,
				MaxBackups: 10,
				MaxAgeDays: 30,
			},
		},
	}
}

func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()
	if err := loadConfigFile(cfg, os.Getenv("CONFIG_PATH")); err != nil {
		return nil, err
	}
	applyStringEnv("JWT_SECRET", &cfg.JWT.Secret)
	applyStringEnv("DB_DSN", &cfg.Database.DSN)
	applyStringEnv("DB_DRIVER", &cfg.Database.Driver)
	applyStringEnv("LISTEN_ADDR", &cfg.Server.Addr)
	applyStringEnv("REDIS_ADDR", &cfg.Redis.Addr)
	applyStringEnv("REDIS_PASSWORD", &cfg.Redis.Password)
	applyIntEnv("REDIS_DB", &cfg.Redis.DB)

	applyLogEnv(cfg)
	applySecurityEnv(cfg)
	applyAdminEnv(cfg)
	applyMonitoringEnv(cfg)
	applyTrustedProxiesEnv(&cfg.Server.TrustedProxies)
	return cfg, nil
}

func (c *Config) IsDefaultSecret() bool {
	return c.JWT.Secret == "change-me-in-production"
}

// IsDemoMode returns true when the application runs in demo/development mode.
func (c *Config) IsDemoMode() bool {
	return c.Security.Mode == "demo"
}

func loadConfigFile(cfg *Config, path string) error {
	if path == "" {
		path = "config.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

func applyLogEnv(cfg *Config) {
	applyStringEnv("LOG_LEVEL", &cfg.Log.Level)
	applyStringEnv("LOG_FORMAT", &cfg.Log.Format)
	applyBoolEnv("LOG_FILE_ENABLED", &cfg.Log.File.Enabled)
	applyStringEnv("LOG_FILE_PATH", &cfg.Log.File.Path)
	applyIntEnv("LOG_FILE_MAX_SIZE_MB", &cfg.Log.File.MaxSizeMB)
	applyIntEnv("LOG_FILE_MAX_BACKUPS", &cfg.Log.File.MaxBackups)
	applyIntEnv("LOG_FILE_MAX_AGE_DAYS", &cfg.Log.File.MaxAgeDays)
	applyBoolEnv("LOG_FILE_COMPRESS", &cfg.Log.File.Compress)
}

func applySecurityEnv(cfg *Config) {
	applyStringEnv("ENCRYPTION_KEY", &cfg.Security.EncryptionKey)
	applyStringEnv("OIDC_SECRET", &cfg.Security.OIDCSecret)
	applyBoolEnv("SECURITY_ALLOW_INSECURE", &cfg.Security.AllowInsecure)
	applyStringEnv("SECURITY_MODE", &cfg.Security.Mode)
}

func applyAdminEnv(cfg *Config) {
	applyStringEnv("ADMIN_USERNAME", &cfg.Admin.Username)
	applyStringEnv("ADMIN_PASSWORD", &cfg.Admin.Password)
	applyStringEnv("ADMIN_EMAIL", &cfg.Admin.Email)
}

func applyMonitoringEnv(cfg *Config) {
	applyStringEnv("MONITORING_AGENT_LATEST_VERSION", &cfg.Monitoring.AgentLatestVersion)
	applyStringEnv("MONITORING_AGENT_INSTALL_COMMAND_TEMPLATE", &cfg.Monitoring.AgentInstallCommandTemplate)
}

func applyTrustedProxiesEnv(target *[]string) {
	v := os.Getenv("TRUSTED_PROXIES")
	if v == "" {
		return
	}
	proxies := make([]string, 0, len(strings.Split(v, ",")))
	for _, raw := range strings.Split(v, ",") {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			proxies = append(proxies, trimmed)
		}
	}
	*target = proxies
}

func applyStringEnv(key string, target *string) {
	if v := os.Getenv(key); v != "" {
		*target = v
	}
}

func applyIntEnv(key string, target *int) {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			*target = n
		}
	}
}

func applyBoolEnv(key string, target *bool) {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			*target = b
		}
	}
}
