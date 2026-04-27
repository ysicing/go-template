package mfahandler

import (
	"testing"

	"github.com/ysicing/go-template/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
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

func createLocalUser(t *testing.T, db *gorm.DB, username, email, password string) *model.User {
	t.Helper()
	user := &model.User{
		Username:   username,
		Email:      email,
		Provider:   "local",
		ProviderID: username,
		InviteCode: "INV-" + username,
	}
	if err := user.SetPassword(password); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}
