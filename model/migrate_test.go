package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type legacyTokenWithoutSubjectFields struct {
	Base
	TokenID      string `gorm:"uniqueIndex;type:varchar(36)"`
	UserID       string `gorm:"type:varchar(36);index"`
	ClientID     string `gorm:"type:varchar(255);index"`
	Scopes       string `gorm:"type:varchar(512)"`
	TokenType    string `gorm:"type:varchar(20);default:'access'"`
	RefreshToken string `gorm:"type:varchar(512);index"`
	ExpiresAt    time.Time
	Revoked      bool `gorm:"default:false"`
}

func (legacyTokenWithoutSubjectFields) TableName() string { return "tokens" }

func TestMigrate_LegacyUsersTableWithoutTokenVersionColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			username TEXT,
			email TEXT,
			password_hash TEXT,
			is_admin BOOLEAN,
			provider TEXT,
			provider_id TEXT,
			avatar_url TEXT,
			email_verified BOOLEAN,
			email_updated_at DATETIME,
			invite_code TEXT,
			invited_by_user_id TEXT,
			invite_ip TEXT,
			permissions TEXT
		)
	`).Error; err != nil {
		t.Fatalf("create legacy users table: %v", err)
	}

	if err := db.Exec("INSERT INTO users (id, username, email, provider, provider_id) VALUES (?, ?, ?, ?, ?)", "u-1", "u1", "u1@example.com", "local", "u1").Error; err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	if !db.Migrator().HasColumn(&User{}, "TokenVersion") {
		t.Fatal("expected token_version column to exist after migrate")
	}

	var got int64
	if err := db.Table("users").Select("token_version").Where("id = ?", "u-1").Scan(&got).Error; err != nil {
		t.Fatalf("query token_version: %v", err)
	}
	if got != 1 {
		t.Fatalf("expected token_version=1 after backfill, got %d", got)
	}
}

func TestMigrate_LegacyRefreshTokenTableWithoutFamilyColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	// Simulate a legacy api_refresh_tokens table that predates the family column.
	if err := db.Exec(`
		CREATE TABLE api_refresh_tokens (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			user_id TEXT,
			token_hash TEXT,
			expires_at DATETIME,
			ip TEXT,
			user_agent TEXT,
			last_used_at DATETIME
		)
	`).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	if !db.Migrator().HasColumn(&APIRefreshToken{}, "Family") {
		t.Fatal("expected family column to exist after migrate")
	}
}

func TestMigrate_AddsExpiryIndexesForTokenCleanup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	type idx struct {
		Name string
	}
	var indexes []idx
	if err := db.Raw("PRAGMA index_list('tokens')").Scan(&indexes).Error; err != nil {
		t.Fatalf("list token indexes: %v", err)
	}
	foundTokenExpiresIdx := false
	for _, it := range indexes {
		if it.Name == "idx_tokens_expires_at" {
			foundTokenExpiresIdx = true
			break
		}
	}
	if !foundTokenExpiresIdx {
		t.Fatal("expected idx_tokens_expires_at to exist")
	}

	indexes = nil
	if err := db.Raw("PRAGMA index_list('api_refresh_tokens')").Scan(&indexes).Error; err != nil {
		t.Fatalf("list api_refresh_tokens indexes: %v", err)
	}
	foundRefreshExpiresIdx := false
	for _, it := range indexes {
		if it.Name == "idx_refresh_tokens_expires_at" {
			foundRefreshExpiresIdx = true
			break
		}
	}
	if !foundRefreshExpiresIdx {
		t.Fatal("expected idx_refresh_tokens_expires_at to exist")
	}
}

func TestMigrate_TemplateSchemaOnlyIncludesRetainedTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	for _, table := range []string{
		"users",
		"oauth_clients",
		"oauth_consent_grants",
		"auth_requests",
		"tokens",
		"signing_keys",
		"social_providers",
		"social_accounts",
		"settings",
		"api_refresh_tokens",
		"password_histories",
		"audit_logs",
		"mfa_configs",
		"webauthn_credentials",
		"user_points",
		"point_transactions",
		"check_in_records",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected retained table %s to exist", table)
		}
	}

	for _, table := range []string{
		"organizations",
		"organization_members",
		"organization_policies",
		"domain_events",
		"service_accounts",
		"service_account_resource_bindings",
		"integration_tokens",
		"quotes",
		"quote_submissions",
		"workspace_plan_assignments",
		"workspace_brandings",
		"webhook_endpoints",
	} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("expected removed table %s to be absent", table)
		}
	}
}

func TestMigrate_AddsTokenSubjectFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&legacyTokenWithoutSubjectFields{}); err != nil {
		t.Fatalf("create legacy tokens table: %v", err)
	}

	if err := db.Create(&legacyTokenWithoutSubjectFields{
		Base:      Base{ID: "tok-1"},
		TokenID:   "token-1",
		UserID:    "user-1",
		ClientID:  "client-1",
		Scopes:    "openid,profile",
		TokenType: "access",
		ExpiresAt: time.Now().Add(time.Hour),
		Revoked:   false,
	}).Error; err != nil {
		t.Fatalf("seed legacy token: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	if !db.Migrator().HasColumn(&Token{}, "subject_type") {
		t.Fatal("expected subject_type column to exist after migrate")
	}
	if !db.Migrator().HasColumn(&Token{}, "subject_id") {
		t.Fatal("expected subject_id column to exist after migrate")
	}

	var got struct {
		SubjectType string
		SubjectID   string
	}
	if err := db.Raw("SELECT subject_type, subject_id FROM tokens WHERE id = ?", "tok-1").Scan(&got).Error; err != nil {
		t.Fatalf("query token subject fields: %v", err)
	}
	if got.SubjectType != "user" {
		t.Fatalf("expected subject_type=user after backfill, got %q", got.SubjectType)
	}
	if got.SubjectID != "user-1" {
		t.Fatalf("expected subject_id=user-1 after backfill, got %q", got.SubjectID)
	}
}
