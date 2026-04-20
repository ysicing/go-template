package app

import (
	"context"
	"io"
	"testing"

	"github.com/ysicing/go-template/store"

	"github.com/rs/zerolog"
)

func TestInitDeps_SetsCache(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Database.Driver = "sqlite"
	cfg.Database.DSN = testSQLiteDSN(t)

	log := zerolog.New(io.Discard)
	ctx := context.Background()

	db, cache := initDBAndCache(ctx, cfg, &log)
	t.Cleanup(func() {
		_ = cache.Close()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	deps := initDeps(ctx, db, cache, cfg, &log)
	if deps.Cache == nil {
		t.Fatal("expected deps.Cache to be initialized")
	}
	if deps.Cache != cache {
		t.Fatal("expected deps.Cache to reuse cache from initDBAndCache")
	}
}

func TestInitDepsInitializesTemplateModules(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Database.Driver = "sqlite"
	cfg.Database.DSN = testSQLiteDSN(t)

	log := zerolog.New(io.Discard)
	ctx := context.Background()

	db, cache := initDBAndCache(ctx, cfg, &log)
	t.Cleanup(func() {
		_ = cache.Close()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	deps := initDeps(ctx, db, cache, cfg, &log)

	if deps.UserStore == nil {
		t.Fatal("expected user store to be initialized")
	}
	if deps.ClientStore == nil {
		t.Fatal("expected oauth client store to be initialized")
	}
	if deps.SettingStore == nil {
		t.Fatal("expected setting store to be initialized")
	}
	if deps.PointStore == nil {
		t.Fatal("expected point store to be initialized")
	}
	if deps.OIDCStorage == nil {
		t.Fatal("expected oidc storage to be initialized")
	}
	if deps.Services.ClientCredentials == nil {
		t.Fatal("expected client credentials service to be initialized")
	}
	if deps.Services.Sessions == nil {
		t.Fatal("expected session service to be initialized")
	}
	if deps.Services.Auth == nil {
		t.Fatal("expected auth service to be initialized")
	}
}

func TestInitCache_FallbacksToMemoryWhenRedisUnavailable(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Redis.Addr = "127.0.0.1:1"

	log := zerolog.New(io.Discard)
	cache := initCache(context.Background(), cfg, &log)
	t.Cleanup(func() { _ = cache.Close() })

	if _, ok := cache.(*store.MemoryCache); !ok {
		t.Fatalf("expected memory cache fallback, got %T", cache)
	}
}

func TestResolveOIDCSecretSource_PrefersExplicitSecret(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Security.OIDCSecret = "oidc-secret"
	cfg.Security.EncryptionKey = "encryption-key"

	got, mode, err := resolveOIDCSecretSource(cfg)
	if err != nil {
		t.Fatalf("resolveOIDCSecretSource() error = %v", err)
	}
	if got != "oidc-secret" || mode != oidcSecretModeExplicit {
		t.Fatalf("resolveOIDCSecretSource() = (%q,%q), want (%q,%q)", got, mode, "oidc-secret", oidcSecretModeExplicit)
	}
}

func TestResolveOIDCSecretSource_FallsBackToEncryptionKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Security.EncryptionKey = "encryption-key"

	got, mode, err := resolveOIDCSecretSource(cfg)
	if err != nil {
		t.Fatalf("resolveOIDCSecretSource() error = %v", err)
	}
	if got != "encryption-key:oidc" || mode != oidcSecretModeEncryptionKey {
		t.Fatalf("resolveOIDCSecretSource() = (%q,%q), want (%q,%q)", got, mode, "encryption-key:oidc", oidcSecretModeEncryptionKey)
	}
}

func TestResolveOIDCSecretSource_RejectsSecureModeWithoutDedicatedSecret(t *testing.T) {
	cfg := DefaultConfig()

	if _, _, err := resolveOIDCSecretSource(cfg); err == nil {
		t.Fatal("expected resolveOIDCSecretSource() to reject secure mode without oidc_secret or encryption_key")
	}
}
