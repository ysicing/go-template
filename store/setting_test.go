package store

import (
	"context"
	crand "crypto/rand"
	"path/filepath"
	"testing"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/crypto"

	"gorm.io/gorm"
)

func newSettingStoreTest(t *testing.T) (*SettingStore, *gorm.DB, Cache) {
	t.Helper()

	db, err := InitDB("sqlite", filepath.Join(t.TempDir(), "settings.db"), "error")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cache := NewMemoryCache()
	t.Cleanup(func() {
		_ = cache.Close()
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return NewSettingStore(db, cache), db, cache
}

func newEncryptedSettingStoreTest(t *testing.T, key string) (*SettingStore, *gorm.DB, Cache) {
	t.Helper()

	db, err := InitDB("sqlite", filepath.Join(t.TempDir(), "settings-encrypted.db"), "error")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cache := NewMemoryCache()
	t.Cleanup(func() {
		_ = cache.Close()
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return NewSettingStore(db, cache, key), db, cache
}

func TestSettingStore_SetWritesThroughCache(t *testing.T) {
	settings, db, _ := newSettingStoreTest(t)
	ctx := context.Background()

	if err := settings.Set(ctx, SettingTurnstileSiteKey, "site-key-1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := db.WithContext(ctx).Where("key = ?", SettingTurnstileSiteKey).Delete(&model.Setting{}).Error; err != nil {
		t.Fatalf("delete backing row: %v", err)
	}

	got := settings.GetWithContext(ctx, SettingTurnstileSiteKey, "")
	if got != "site-key-1" {
		t.Fatalf("expected cached setting value, got %q", got)
	}
}

func TestSettingStore_DeleteCachesMiss(t *testing.T) {
	settings, db, _ := newSettingStoreTest(t)
	ctx := context.Background()

	if err := settings.Set(ctx, SettingTurnstileSiteKey, "site-key-1"); err != nil {
		t.Fatalf("seed setting: %v", err)
	}
	if err := settings.Delete(ctx, SettingTurnstileSiteKey); err != nil {
		t.Fatalf("delete setting: %v", err)
	}
	if err := db.WithContext(ctx).Create(&model.Setting{Key: SettingTurnstileSiteKey, Value: "site-key-2"}).Error; err != nil {
		t.Fatalf("recreate backing row: %v", err)
	}

	got := settings.GetWithContext(ctx, SettingTurnstileSiteKey, "")
	if got != "" {
		t.Fatalf("expected cached miss after delete, got %q", got)
	}
}

func TestSettingStore_EncryptsSecretValuesAtRest(t *testing.T) {
	settings, db, _ := newEncryptedSettingStoreTest(t, "encryption-key")
	ctx := context.Background()

	if err := settings.Set(ctx, SettingSMTPPassword, "smtp-secret"); err != nil {
		t.Fatalf("set: %v", err)
	}

	var raw model.Setting
	if err := db.WithContext(ctx).Where("key = ?", SettingSMTPPassword).First(&raw).Error; err != nil {
		t.Fatalf("query raw setting: %v", err)
	}
	if raw.Value == "smtp-secret" {
		t.Fatal("expected smtp password to be encrypted at rest")
	}
	if !crypto.IsEncrypted(raw.Value) {
		t.Fatalf("expected encrypted value prefix, got %q", raw.Value)
	}
	if got := settings.GetWithContext(ctx, SettingSMTPPassword, ""); got != "smtp-secret" {
		t.Fatalf("expected decrypted smtp password, got %q", got)
	}
}

func TestSettingStore_SetFailsWhenSecretEncryptionFails(t *testing.T) {
	settings, db, _ := newEncryptedSettingStoreTest(t, "encryption-key")
	ctx := context.Background()

	originalReader := crand.Reader
	crand.Reader = failingReader{}
	t.Cleanup(func() { crand.Reader = originalReader })

	err := settings.Set(ctx, SettingSMTPPassword, "smtp-secret")
	if err == nil {
		t.Fatal("expected encryption failure to be returned")
	}

	var count int64
	if err := db.WithContext(ctx).Model(&model.Setting{}).Where("key = ?", SettingSMTPPassword).Count(&count).Error; err != nil {
		t.Fatalf("count setting rows: %v", err)
	}
	if count != 0 {
		t.Fatal("expected setting not to be persisted when encryption fails")
	}
}

func TestSettingStore_GetReturnsEmptySecretWhenDecryptFails(t *testing.T) {
	writer, _, _ := newEncryptedSettingStoreTest(t, "writer-key")
	ctx := context.Background()

	if err := writer.Set(ctx, SettingSMTPPassword, "smtp-secret"); err != nil {
		t.Fatalf("seed encrypted setting: %v", err)
	}

	reader, _, _ := newEncryptedSettingStoreTest(t, "wrong-reader-key")
	reader.db = writer.db
	reader.cache = NewMemoryCache()
	t.Cleanup(func() { _ = reader.cache.Close() })

	if got := reader.GetWithContext(ctx, SettingSMTPPassword, ""); got != "" {
		t.Fatalf("expected empty secret on decrypt failure, got %q", got)
	}
}

func TestSettingStore_GetReturnsEmptySecretForPlaintextLegacyValue(t *testing.T) {
	writer, db, _ := newEncryptedSettingStoreTest(t, "writer-key")
	ctx := context.Background()

	if err := db.WithContext(ctx).Create(&model.Setting{Key: SettingSMTPPassword, Value: "legacy-plain-secret"}).Error; err != nil {
		t.Fatalf("seed plaintext setting: %v", err)
	}

	reader := NewSettingStore(db, writer.cache, "reader-key")
	if got := reader.GetWithContext(ctx, SettingSMTPPassword, ""); got != "" {
		t.Fatalf("expected empty secret for legacy plaintext value, got %q", got)
	}
}

func TestSettingStore_SetManyIsAtomicWhenSecretEncryptionFails(t *testing.T) {
	settings, db, _ := newEncryptedSettingStoreTest(t, "encryption-key")
	ctx := context.Background()

	originalReader := crand.Reader
	crand.Reader = failingReader{}
	t.Cleanup(func() { crand.Reader = originalReader })

	err := settings.SetMany(ctx, map[string]string{
		SettingSiteTitle:    "Acme ID",
		SettingSMTPPassword: "smtp-secret",
	})
	if err == nil {
		t.Fatal("expected batch encryption failure to be returned")
	}

	var count int64
	if err := db.WithContext(ctx).Model(&model.Setting{}).Where("key IN ?", []string{SettingSiteTitle, SettingSMTPPassword}).Count(&count).Error; err != nil {
		t.Fatalf("count settings: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no settings to be persisted after rollback, got %d", count)
	}
}
