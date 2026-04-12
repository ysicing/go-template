package app

import "testing"

func TestDefaultConfigUsesPort3206(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Addr != ":3206" {
		t.Fatalf("expected default server addr %q, got %q", ":3206", cfg.Server.Addr)
	}
}

func TestLoadConfigAppliesEnvOverridesWithoutConfigFile(t *testing.T) {
	t.Setenv("CONFIG_PATH", t.TempDir()+"/missing-config.yaml")
	t.Setenv("JWT_SECRET", "dev-local-secret")
	t.Setenv("SECURITY_ALLOW_INSECURE", "true")
	t.Setenv("DB_DSN", "file:test.db")

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
}
