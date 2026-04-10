package setup

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/cache"
	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
	"github.com/ysicing/go-template/internal/system"
	"github.com/ysicing/go-template/internal/user"
	"gorm.io/gorm"
)

type InstallInput struct {
	Server        config.ServerConfig   `json:"server"`
	Log           config.LogConfig      `json:"log"`
	JWT           config.JWTConfig      `json:"jwt"`
	Database      config.DatabaseConfig `json:"database"`
	Cache         config.CacheConfig    `json:"cache"`
	AdminUsername string                `json:"admin_username"`
	AdminEmail    string                `json:"admin_email"`
	AdminPassword string                `json:"admin_password"`
}

type Service struct {
	ConfigPath string
	DB         *gorm.DB
}

func NewService(configPath string, conn *gorm.DB) *Service {
	return &Service{ConfigPath: configPath, DB: conn}
}

func (s *Service) SetupRequired() (bool, error) {
	if s.DB == nil {
		return true, nil
	}

	var count int64
	if err := s.DB.Model(&system.BootstrapState{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *Service) Install(ctx context.Context, input InstallInput) error {
	if err := validateInput(input); err != nil {
		return err
	}

	cfg := buildConfig(input)
	conn, store, err := prepareDependencies(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeStore(store)

	if err := s.installState(conn, input); err != nil {
		return err
	}

	return config.Save(s.ConfigPath, cfg)
}

func buildConfig(input InstallInput) *config.Config {
	cfg := config.Default()
	cfg.Server = input.Server
	cfg.Log = input.Log
	cfg.JWT = input.JWT
	cfg.Database = input.Database
	cfg.Cache = input.Cache
	return cfg
}

func prepareDependencies(ctx context.Context, cfg *config.Config) (*gorm.DB, cache.Store, error) {
	conn, err := db.Open(cfg.Database)
	if err != nil {
		return nil, nil, err
	}
	if err := db.AutoMigrate(conn); err != nil {
		return nil, nil, err
	}

	store, err := cache.NewStore(ctx, cfg.Cache)
	if err != nil {
		return nil, nil, err
	}

	return conn, store, nil
}

func closeStore(store cache.Store) {
	type closer interface {
		Close() error
	}
	if closerStore, ok := store.(closer); ok {
		_ = closerStore.Close()
	}
}

func validateInput(input InstallInput) error {
	if strings.TrimSpace(input.AdminUsername) == "" {
		return errors.New("admin username is required")
	}
	if strings.TrimSpace(input.AdminEmail) == "" {
		return errors.New("admin email is required")
	}
	if len(strings.TrimSpace(input.AdminPassword)) < 8 {
		return errors.New("admin password must be at least 8 characters")
	}
	if strings.TrimSpace(input.Database.Driver) == "" {
		return errors.New("database driver is required")
	}
	if strings.TrimSpace(input.Database.DSN) == "" {
		return errors.New("database dsn is required")
	}
	if strings.TrimSpace(input.Server.Host) == "" || input.Server.Port == 0 {
		return errors.New("server host and port are required")
	}
	if strings.TrimSpace(input.JWT.Secret) == "" {
		return errors.New("jwt secret is required")
	}
	if strings.TrimSpace(input.JWT.AccessTTL.String()) == "" || strings.TrimSpace(input.JWT.RefreshTTL.String()) == "" {
		return errors.New("jwt ttl is required")
	}
	return nil
}

func (s *Service) installState(conn *gorm.DB, input InstallInput) error {
	return conn.Transaction(func(tx *gorm.DB) error {
		hash, err := auth.HashPassword(input.AdminPassword)
		if err != nil {
			return err
		}

		admin := user.User{
			Username:     input.AdminUsername,
			Email:        input.AdminEmail,
			PasswordHash: hash,
			Role:         user.RoleAdmin,
			Status:       "active",
		}
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}

		settings := system.DefaultSettings()
		if err := tx.Create(&settings).Error; err != nil {
			return err
		}

		state := system.BootstrapState{
			InitializedAt: time.Now().UTC(),
			Version:       "v1",
		}
		return tx.Create(&state).Error
	})
}
