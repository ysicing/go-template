package db_test

import (
	"testing"

	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
)

func TestDialectorForSQLite(t *testing.T) {
	cfg := config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared&_pragma=foreign_keys(1)",
	}

	dialector, err := db.NewDialector(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dialector == nil {
		t.Fatal("expected non-nil dialector")
	}
}

func TestAutoMigrateModels(t *testing.T) {
	conn, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared&_pragma=foreign_keys(1)",
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := db.AutoMigrate(conn); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
}

