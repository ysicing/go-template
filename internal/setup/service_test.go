package setup_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
	"github.com/ysicing/go-template/internal/setup"
)

func TestStatusRequiredWhenBootstrapMissing(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "status.db") + "?_pragma=foreign_keys(1)"
	conn, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    dsn,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(conn); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	svc := setup.NewService(filepath.Join(t.TempDir(), "config.yaml"), conn)
	required, err := svc.SetupRequired()
	if err != nil {
		t.Fatalf("setup required: %v", err)
	}
	if !required {
		t.Fatal("expected setup required")
	}
}

func TestInstallCreatesAdminAndBootstrapState(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "install.db")

	svc := setup.NewService(cfgPath, nil)
	input := setup.InstallInput{
		Server:        config.ServerConfig{Host: "0.0.0.0", Port: 3206},
		Log:           config.LogConfig{Level: "info"},
		JWT:           config.JWTConfig{Issuer: "go-template", AccessTTL: "15m", RefreshTTL: "168h", Secret: "secret123"},
		Database:      config.DatabaseConfig{Driver: "sqlite", DSN: "file:" + dbPath + "?_pragma=foreign_keys(1)"},
		Cache:         config.CacheConfig{Driver: "memory"},
		AdminUsername: "admin",
		AdminEmail:    "admin@example.com",
		AdminPassword: "secret123",
	}

	if err := svc.Install(context.Background(), input); err != nil {
		t.Fatalf("install: %v", err)
	}

	conn, err := db.Open(input.Database)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	postInstall := setup.NewService(cfgPath, conn)
	required, err := postInstall.SetupRequired()
	if err != nil {
		t.Fatalf("setup required after install: %v", err)
	}
	if required {
		t.Fatal("expected setup completed")
	}

	if _, err := config.Load(cfgPath); err != nil {
		t.Fatalf("load config: %v", err)
	}
}
