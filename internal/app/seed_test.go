package app

import (
	"context"
	"io"
	"testing"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func setupUserStoreTest(t *testing.T) *store.UserStore {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return store.NewUserStore(db)
}

func TestSeedAdmin_AllowsWeakPasswordByDefault(t *testing.T) {
	users := setupUserStoreTest(t)
	log := zerolog.New(io.Discard)

	seedAdmin(&log, users, AdminSeedConfig{
		Username: "admin",
		Password: "weak",
		Email:    "admin@example.com",
	})

	user, err := users.GetByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("expected admin to be created, got err=%v", err)
	}
	if !user.CheckPassword("weak") {
		t.Fatal("expected seeded admin password to match configured password")
	}
}

func TestSeedAdmin_SkipsEmptyPassword(t *testing.T) {
	users := setupUserStoreTest(t)
	log := zerolog.New(io.Discard)

	seedAdmin(&log, users, AdminSeedConfig{
		Username: "admin",
		Email:    "admin@example.com",
	})

	if _, err := users.GetByUsername(context.Background(), "admin"); err == nil {
		t.Fatal("expected admin user not to be created without password")
	}
}

func TestSeedAdmin_CreatesUserWithStrongPassword(t *testing.T) {
	users := setupUserStoreTest(t)
	log := zerolog.New(io.Discard)

	seedAdmin(&log, users, AdminSeedConfig{
		Username: "admin",
		Password: "StrongPass123!@#",
		Email:    "admin@example.com",
	})

	user, err := users.GetByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("expected admin user to be created, got err=%v", err)
	}
	if !user.IsAdmin {
		t.Fatal("expected seeded user to be admin")
	}
	if !user.CheckPassword("StrongPass123!@#") {
		t.Fatal("expected seeded admin password to match configured password")
	}
}
