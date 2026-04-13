package store

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

func newSettingStoreTest(t *testing.T) (*SettingStore, *gorm.DB, Cache) {
	t.Helper()

	db, err := InitDB("sqlite", "file::memory:?cache=shared", "error")
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
