package store

import (
	"path/filepath"
	"testing"

	"github.com/ysicing/go-template/model"
)

func TestVerifySQLiteWritable_AllowsWritableDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := InitDB("sqlite", dbPath, "error")
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := VerifySQLiteWritable(db); err != nil {
		t.Fatalf("VerifySQLiteWritable() error = %v", err)
	}
}

func TestVerifySQLiteWritable_RejectsReadonlyDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	writable, err := InitDB("sqlite", dbPath, "error")
	if err != nil {
		t.Fatalf("InitDB() writable error = %v", err)
	}
	if err := model.Migrate(writable); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	readonly, err := InitDB("sqlite", "file:"+dbPath+"?mode=ro", "error")
	if err != nil {
		t.Fatalf("InitDB() readonly error = %v", err)
	}
	if err := VerifySQLiteWritable(readonly); err == nil {
		t.Fatal("expected VerifySQLiteWritable() to fail for readonly sqlite database")
	}
}
