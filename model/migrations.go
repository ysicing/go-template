package model

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func migrations() []*gormigrate.Migration {
	return []*gormigrate.Migration{
		{
			// Backfill empty provider_id for existing local users
			// so the (provider, provider_id) unique index can be created.
			ID: "202602281500_backfill_provider_id",
			Migrate: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("users") {
					return nil
				}
				return tx.Exec(
					"UPDATE users SET provider_id = username WHERE provider = 'local' AND (provider_id IS NULL OR provider_id = '')",
				).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return nil // no safe rollback
			},
		},
		{
			// Add composite indexes for audit logs to improve query performance
			ID: "202603011600_add_audit_log_indexes",
			Migrate: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("audit_logs") {
					return nil
				}
				// Index for login history queries (action + created_at)
				// PostgreSQL supports partial indexes with WHERE clause
				// MySQL doesn't support WHERE in CREATE INDEX, so we create a regular index
				var sql string
				if tx.Dialector.Name() == "postgres" {
					sql = "CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created ON audit_logs(action, created_at DESC) WHERE deleted_at IS NULL"
				} else {
					sql = "CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created ON audit_logs(action, created_at DESC)"
				}
				if err := tx.Exec(sql).Error; err != nil {
					return err
				}

				// Index for user action queries (user_id + action + created_at)
				if tx.Dialector.Name() == "postgres" {
					sql = "CREATE INDEX IF NOT EXISTS idx_audit_logs_user_action ON audit_logs(user_id, action, created_at DESC) WHERE deleted_at IS NULL"
				} else {
					sql = "CREATE INDEX IF NOT EXISTS idx_audit_logs_user_action ON audit_logs(user_id, action, created_at DESC)"
				}
				if err := tx.Exec(sql).Error; err != nil {
					return err
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("audit_logs") {
					return nil
				}
				_ = tx.Exec("DROP INDEX IF EXISTS idx_audit_logs_action_created").Error
				_ = tx.Exec("DROP INDEX IF EXISTS idx_audit_logs_user_action").Error
				return nil
			},
		},
		{
			// Add index for refresh token family queries
			ID: "202603011601_add_refresh_token_family_index",
			Migrate: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("api_refresh_tokens") {
					return nil
				}
				// Legacy deployments may have the table without the Family column because
				// gormigrate runs before AutoMigrate.
				if !tx.Migrator().HasColumn(&APIRefreshToken{}, "Family") {
					if err := tx.Migrator().AddColumn(&APIRefreshToken{}, "Family"); err != nil {
						return err
					}
				}
				var sql string
				if tx.Dialector.Name() == "postgres" {
					sql = "CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON api_refresh_tokens(family) WHERE deleted_at IS NULL"
				} else {
					sql = "CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family ON api_refresh_tokens(family)"
				}
				return tx.Exec(sql).Error
			},
			Rollback: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("api_refresh_tokens") {
					return nil
				}
				return tx.Exec("DROP INDEX IF EXISTS idx_refresh_tokens_family").Error
			},
		},
		{
			// Add expires_at indexes to speed up expired token cleanup jobs.
			ID: "202603031200_add_token_expiry_indexes",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable("tokens") {
					if tx.Dialector.Name() == "postgres" {
						if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at) WHERE deleted_at IS NULL").Error; err != nil {
							return err
						}
					} else {
						if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at)").Error; err != nil {
							return err
						}
					}
				}

				if tx.Migrator().HasTable("api_refresh_tokens") {
					if tx.Dialector.Name() == "postgres" {
						if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON api_refresh_tokens(expires_at) WHERE deleted_at IS NULL").Error; err != nil {
							return err
						}
					} else {
						if err := tx.Exec("CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON api_refresh_tokens(expires_at)").Error; err != nil {
							return err
						}
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable("tokens") {
					_ = tx.Exec("DROP INDEX IF EXISTS idx_tokens_expires_at").Error
				}
				if tx.Migrator().HasTable("api_refresh_tokens") {
					_ = tx.Exec("DROP INDEX IF EXISTS idx_refresh_tokens_expires_at").Error
				}
				return nil
			},
		},
		{
			ID: "202603041100_add_user_token_version",
			Migrate: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("users") {
					return nil
				}
				if !tx.Migrator().HasColumn(&User{}, "TokenVersion") {
					if err := tx.Migrator().AddColumn(&User{}, "TokenVersion"); err != nil {
						return err
					}
				}
				return tx.Exec("UPDATE users SET token_version = 1 WHERE token_version IS NULL OR token_version < 1").Error
			},
			Rollback: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("users") {
					return nil
				}
				if !tx.Migrator().HasColumn(&User{}, "TokenVersion") {
					return nil
				}
				return tx.Migrator().DropColumn(&User{}, "TokenVersion")
			},
		},
		{
			// Add email_updated_at field to track email change frequency
			ID: "202603021600_add_email_updated_at",
			Migrate: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("users") {
					return nil
				}
				if tx.Migrator().HasColumn(&User{}, "EmailUpdatedAt") {
					return nil
				}
				return tx.Migrator().AddColumn(&User{}, "EmailUpdatedAt")
			},
			Rollback: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("users") {
					return nil
				}
				return tx.Migrator().DropColumn(&User{}, "EmailUpdatedAt")
			},
		},
		{
			// Create social_accounts table for multiple social login bindings
			ID: "202603021700_create_social_accounts",
			Migrate: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable("social_accounts") {
					return nil
				}
				if err := tx.AutoMigrate(&SocialAccount{}); err != nil {
					return err
				}
				// Create unique index on provider + provider_id.
				// PostgreSQL and SQLite support partial indexes (WHERE deleted_at IS NULL)
				// which allows re-creating soft-deleted accounts.
				// MySQL does not support partial indexes, so a regular index is used.
				var sql string
				if tx.Dialector.Name() == "mysql" {
					sql = "CREATE UNIQUE INDEX IF NOT EXISTS idx_social_accounts_provider_id ON social_accounts(provider, provider_id)"
				} else {
					sql = "CREATE UNIQUE INDEX IF NOT EXISTS idx_social_accounts_provider_id ON social_accounts(provider, provider_id) WHERE deleted_at IS NULL"
				}
				return tx.Exec(sql).Error
			},
			Rollback: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("social_accounts")
			},
		},
		{
			ID: "202604011000_add_social_account_profile_fields",
			Migrate: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("social_accounts") {
					return nil
				}
				if !tx.Migrator().HasColumn(&SocialAccount{}, "Username") {
					if err := tx.Migrator().AddColumn(&SocialAccount{}, "Username"); err != nil {
						return err
					}
				}
				if !tx.Migrator().HasColumn(&SocialAccount{}, "DisplayName") {
					if err := tx.Migrator().AddColumn(&SocialAccount{}, "DisplayName"); err != nil {
						return err
					}
				}
				return nil
			},
			Rollback: func(tx *gorm.DB) error {
				if tx.Migrator().HasTable("social_accounts") && tx.Migrator().HasColumn(&SocialAccount{}, "DisplayName") {
					if err := tx.Migrator().DropColumn(&SocialAccount{}, "DisplayName"); err != nil {
						return err
					}
				}
				if tx.Migrator().HasTable("social_accounts") && tx.Migrator().HasColumn(&SocialAccount{}, "Username") {
					return tx.Migrator().DropColumn(&SocialAccount{}, "Username")
				}
				return nil
			},
		},
		{
			ID: "202604061200_add_token_subject_fields",
			Migrate: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("tokens") {
					return nil
				}
				if !tx.Migrator().HasColumn(&Token{}, "SubjectType") {
					if err := tx.Migrator().AddColumn(&Token{}, "SubjectType"); err != nil {
						return err
					}
				}
				if !tx.Migrator().HasColumn(&Token{}, "SubjectID") {
					if err := tx.Migrator().AddColumn(&Token{}, "SubjectID"); err != nil {
						return err
					}
				}
				return tx.Exec(`
					UPDATE tokens
					SET subject_type = 'user',
					    subject_id = user_id
					WHERE ((subject_type IS NULL OR subject_type = '')
					    OR (subject_id IS NULL OR subject_id = ''))
					  AND user_id IS NOT NULL
					  AND user_id != ''
				`).Error
			},
			Rollback: func(tx *gorm.DB) error {
				if !tx.Migrator().HasTable("tokens") {
					return nil
				}
				if tx.Migrator().HasColumn(&Token{}, "SubjectID") {
					if err := tx.Migrator().DropColumn(&Token{}, "SubjectID"); err != nil {
						return err
					}
				}
				if tx.Migrator().HasColumn(&Token{}, "SubjectType") {
					return tx.Migrator().DropColumn(&Token{}, "SubjectType")
				}
				return nil
			},
		},
	}
}
