package auth_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
	"github.com/ysicing/go-template/internal/user"
)

func TestPasswordHashAndCompare(t *testing.T) {
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := auth.CheckPassword(hash, "secret123"); err != nil {
		t.Fatalf("check password: %v", err)
	}
}

func TestTokenPairIssueAndParse(t *testing.T) {
	manager := auth.NewTokenManager("issuer", "secret", 0, 0)
	manager = auth.NewTokenManager("issuer", "secret", config.Duration("1m").Value(), config.Duration("1h").Value())

	pair, err := manager.Issue(1, "admin")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := manager.ParseAccess(pair.AccessToken)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.UserID != 1 {
		t.Fatalf("expected user id 1, got %d", claims.UserID)
	}
}

func TestServiceLogin(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "auth.db") + "?_pragma=foreign_keys(1)"
	conn, err := db.Open(config.DatabaseConfig{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(conn); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	passwordHash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	account := user.User{
		Username:     "admin",
		Email:        "admin@example.com",
		PasswordHash: passwordHash,
		Role:         user.RoleAdmin,
		Status:       "active",
	}
	if err := conn.Create(&account).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := auth.NewService(conn, auth.NewTokenManager("issuer", "secret", config.Duration("1m").Value(), config.Duration("1h").Value()))
	current, pair, err := service.Login("admin", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if current.Username != "admin" {
		t.Fatalf("expected admin, got %s", current.Username)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected non-empty token pair")
	}
}

func TestServiceLoginRejectsDisabledUser(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "auth.db") + "?_pragma=foreign_keys(1)"
	conn, err := db.Open(config.DatabaseConfig{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(conn); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	passwordHash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	account := user.User{
		Username:     "disabled-admin",
		Email:        "disabled@example.com",
		PasswordHash: passwordHash,
		Role:         user.RoleAdmin,
		Status:       "disabled",
	}
	if err := conn.Create(&account).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := auth.NewService(conn, auth.NewTokenManager("issuer", "secret", config.Duration("1m").Value(), config.Duration("1h").Value()))
	_, pair, err := service.Login("disabled-admin", "secret123")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if pair.AccessToken != "" || pair.RefreshToken != "" {
		t.Fatal("expected empty token pair")
	}
}
