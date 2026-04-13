package app

import "testing"

func TestDefaultConfigUsesPort3206(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Addr != ":3206" {
		t.Fatalf("expected default server addr %q, got %q", ":3206", cfg.Server.Addr)
	}
	if cfg.Log.File.Enabled {
		t.Fatal("expected file logging disabled by default")
	}
	if cfg.Log.File.MaxSizeMB != 100 {
		t.Fatalf("expected default log file max size 100MB, got %d", cfg.Log.File.MaxSizeMB)
	}
}

func TestLoadConfigAppliesEnvOverridesWithoutConfigFile(t *testing.T) {
	t.Setenv("CONFIG_PATH", t.TempDir()+"/missing-config.yaml")
	t.Setenv("JWT_SECRET", "dev-local-secret")
	t.Setenv("SECURITY_ALLOW_INSECURE", "true")
	t.Setenv("DB_DSN", "file:test.db")
	t.Setenv("LOG_FILE_ENABLED", "true")
	t.Setenv("LOG_FILE_PATH", "/tmp/go-template.log")
	t.Setenv("LOG_FILE_MAX_SIZE_MB", "32")
	t.Setenv("LOG_FILE_MAX_BACKUPS", "5")
	t.Setenv("LOG_FILE_MAX_AGE_DAYS", "14")
	t.Setenv("LOG_FILE_COMPRESS", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.JWT.Secret != "dev-local-secret" {
		t.Fatalf("expected env JWT secret to apply, got %q", cfg.JWT.Secret)
	}
	if !cfg.Security.AllowInsecure {
		t.Fatal("expected SECURITY_ALLOW_INSECURE env override to apply")
	}
	if cfg.Database.DSN != "file:test.db" {
		t.Fatalf("expected DB_DSN env override to apply, got %q", cfg.Database.DSN)
	}
	if !cfg.Log.File.Enabled {
		t.Fatal("expected LOG_FILE_ENABLED env override to apply")
	}
	if cfg.Log.File.Path != "/tmp/go-template.log" {
		t.Fatalf("expected LOG_FILE_PATH env override to apply, got %q", cfg.Log.File.Path)
	}
	if cfg.Log.File.MaxSizeMB != 32 || cfg.Log.File.MaxBackups != 5 || cfg.Log.File.MaxAgeDays != 14 {
		t.Fatalf("expected log file rotation env overrides to apply, got %+v", cfg.Log.File)
	}
	if !cfg.Log.File.Compress {
		t.Fatal("expected LOG_FILE_COMPRESS env override to apply")
	}
}
