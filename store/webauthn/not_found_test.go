package webauthnstore

import (
	"context"
	"errors"
	"testing"

	"github.com/ysicing/go-template/model"
	rootstore "github.com/ysicing/go-template/store"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupWebAuthnTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestGetByID_NormalizesNotFound(t *testing.T) {
	s := NewWebAuthnStore(setupWebAuthnTestDB(t))

	if _, err := s.GetByID(context.Background(), "missing-credential"); !errors.Is(err, rootstore.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
