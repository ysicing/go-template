package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ysicing/go-template/internal/config"
)

func TestLoadMissingFile(t *testing.T) {
	_, err := config.Load("testdata/not-found.yaml")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestLoadWithEnvOverride(t *testing.T) {
	t.Setenv("APP_SERVER_PORT", "9090")

	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Server.Port)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Default()
	cfg.Server.Port = 7070
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}

	if saved.Server.Port != 7070 {
		t.Fatalf("expected 7070, got %d", saved.Server.Port)
	}
}

func TestSaveCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.yaml")
	if err := config.Save(path, config.Default()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat config: %v", err)
	}
}

