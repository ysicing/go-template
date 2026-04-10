package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultPath = "configs/config.yaml"

func DefaultPath() string {
	if value := strings.TrimSpace(os.Getenv("APP_CONFIG_PATH")); value != "" {
		return value
	}
	return defaultPath
}

type Duration string

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var value string
	if err := node.Decode(&value); err != nil {
		return err
	}
	if _, err := time.ParseDuration(value); err != nil {
		return fmt.Errorf("parse duration %q: %w", value, err)
	}
	*d = Duration(value)
	return nil
}

func (d Duration) String() string {
	return string(d)
}

func (d Duration) Value() time.Duration {
	parsed, err := time.ParseDuration(string(d))
	if err != nil {
		return 0
	}
	return parsed
}

type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	Log      LogConfig      `yaml:"log" json:"log"`
	JWT      JWTConfig      `yaml:"jwt" json:"jwt"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	Cache    CacheConfig    `yaml:"cache" json:"cache"`
}

type ServerConfig struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

type LogConfig struct {
	Level string `yaml:"level" json:"level"`
}

type JWTConfig struct {
	Issuer     string   `yaml:"issuer" json:"issuer"`
	AccessTTL  Duration `yaml:"access_ttl" json:"access_ttl"`
	RefreshTTL Duration `yaml:"refresh_ttl" json:"refresh_ttl"`
	Secret     string   `yaml:"secret" json:"secret"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver" json:"driver"`
	DSN    string `yaml:"dsn" json:"dsn"`
}

type CacheConfig struct {
	Driver   string `yaml:"driver" json:"driver"`
	Addr     string `yaml:"addr" json:"addr"`
	Password string `yaml:"password" json:"password"`
	DB       int    `yaml:"db" json:"db"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{Host: "0.0.0.0", Port: 3206},
		Log:    LogConfig{Level: "info"},
		JWT: JWTConfig{
			Issuer:     "go-template",
			AccessTTL:  Duration("15m"),
			RefreshTTL: Duration("168h"),
			Secret:     "change-me",
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "file:data/app.db?_pragma=foreign_keys(1)",
		},
		Cache: CacheConfig{Driver: "memory"},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	applyEnv(cfg)
	return cfg, nil
}

func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func applyEnv(cfg *Config) {
	applyString("APP_SERVER_HOST", &cfg.Server.Host)
	applyInt("APP_SERVER_PORT", &cfg.Server.Port)
	applyString("APP_LOG_LEVEL", &cfg.Log.Level)
	applyString("APP_JWT_ISSUER", &cfg.JWT.Issuer)
	applyDuration("APP_JWT_ACCESS_TTL", &cfg.JWT.AccessTTL)
	applyDuration("APP_JWT_REFRESH_TTL", &cfg.JWT.RefreshTTL)
	applyString("APP_JWT_SECRET", &cfg.JWT.Secret)
	applyString("APP_DATABASE_DRIVER", &cfg.Database.Driver)
	applyString("APP_DATABASE_DSN", &cfg.Database.DSN)
	applyString("APP_CACHE_DRIVER", &cfg.Cache.Driver)
	applyString("APP_CACHE_ADDR", &cfg.Cache.Addr)
	applyString("APP_CACHE_PASSWORD", &cfg.Cache.Password)
	applyInt("APP_CACHE_DB", &cfg.Cache.DB)
}

func applyString(key string, target *string) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		*target = value
	}
}

func applyInt(key string, target *int) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			*target = parsed
		}
	}
}

func applyDuration(key string, target *Duration) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if _, err := time.ParseDuration(value); err == nil {
			*target = Duration(value)
		}
	}
}
