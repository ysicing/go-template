package model

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// Migrate runs versioned data migrations first, then auto-migrates schema.
// Order matters: data fixes (e.g. backfill) must complete before schema
// changes (e.g. unique indexes) that depend on clean data.
func Migrate(db *gorm.DB) error {
	m := gormigrate.New(db, gormigrate.DefaultOptions, migrations())
	if err := m.Migrate(); err != nil {
		return err
	}
	if err := db.AutoMigrate(allModels()...); err != nil {
		return err
	}
	return ensureTokenExpiryIndexes(db)
}

func ensureTokenExpiryIndexes(db *gorm.DB) error {
	if db.Migrator().HasTable("tokens") {
		var sql string
		if db.Dialector.Name() == "postgres" {
			sql = "CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at) WHERE deleted_at IS NULL"
		} else {
			sql = "CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at)"
		}
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}

	if db.Migrator().HasTable("api_refresh_tokens") {
		var sql string
		if db.Dialector.Name() == "postgres" {
			sql = "CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON api_refresh_tokens(expires_at) WHERE deleted_at IS NULL"
		} else {
			sql = "CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON api_refresh_tokens(expires_at)"
		}
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}

	return nil
}

func allModels() []any {
	return []any{
		&User{},
		&OAuthClient{},
		&OAuthConsentGrant{},
		&AuthRequest{},
		&Token{},
		&SigningKey{},
		&SocialProvider{},
		&SocialAccount{},
		&Setting{},
		&APIRefreshToken{},
		&PasswordHistory{},
		&AuditLog{},
		&MFAConfig{},
		&WebAuthnCredential{},
		&UserPoints{},
		&PointTransaction{},
		&CheckInRecord{},
	}
}
