package db

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/system"
	"github.com/ysicing/go-template/internal/user"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDialector(cfg config.DatabaseConfig) (gorm.Dialector, error) {
	switch cfg.Driver {
	case "sqlite":
		return sqlite.Open(cfg.DSN), nil
	case "postgres":
		return postgres.Open(cfg.DSN), nil
	case "mysql":
		return mysql.Open(cfg.DSN), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dialector, err := NewDialector(cfg)
	if err != nil {
		return nil, err
	}

	return gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
}

func AutoMigrate(conn *gorm.DB) error {
	return conn.AutoMigrate(
		&user.User{},
		&auth.PasswordResetToken{},
		&system.BootstrapState{},
		&system.Setting{},
	)
}
