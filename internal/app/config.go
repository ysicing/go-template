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
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
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
			DSN:    "id.db",
		},
		JWT: JWTConfig{
			Secret:          "change-me-in-production",
			Issuer:          "id",
			AccessTokenTTL:  1 * time.Hour,       // Access token valid for 1 hour
			RefreshTokenTTL: 7 * 24 * time.Hour,  // Refresh token valid for 7 days
			RememberMeTTL:   30 * 24 * time.Hour, // Remember me: 30 days
		},
		Security: SecurityConfig{
			AllowInsecure: false,
		},
		Log: LogConfig{
			Level:  "info", // WARNING: "info" level logs full SQL queries which may contain sensitive data. Use "warn" or "error" in production.
			Format: "json",
		},
	}
}

func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Redis.DB = n
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := os.Getenv("ENCRYPTION_KEY"); v != "" {
		cfg.Security.EncryptionKey = v
	}
	if v := os.Getenv("OIDC_SECRET"); v != "" {
		cfg.Security.OIDCSecret = v
	}
	if v := os.Getenv("SECURITY_ALLOW_INSECURE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Security.AllowInsecure = b
		}
	}
	if v := os.Getenv("ADMIN_USERNAME"); v != "" {
		cfg.Admin.Username = v
	}
	if v := os.Getenv("ADMIN_PASSWORD"); v != "" {
		cfg.Admin.Password = v
	}
	if v := os.Getenv("ADMIN_EMAIL"); v != "" {
		cfg.Admin.Email = v
	}
	if v := os.Getenv("TRUSTED_PROXIES"); v != "" {
		// Parse comma-separated CIDR list
		proxies := []string{}
		for _, p := range strings.Split(v, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				proxies = append(proxies, trimmed)
			}
		}
		cfg.Server.TrustedProxies = proxies
	}
	if v := os.Getenv("MONITORING_AGENT_LATEST_VERSION"); v != "" {
		cfg.Monitoring.AgentLatestVersion = v
	}
	if v := os.Getenv("MONITORING_AGENT_INSTALL_COMMAND_TEMPLATE"); v != "" {
		cfg.Monitoring.AgentInstallCommandTemplate = v
	}

	return cfg, nil
}

func (c *Config) IsDefaultSecret() bool {
	return c.JWT.Secret == "change-me-in-production"
}
