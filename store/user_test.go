package store

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

func setupUserStoreTestDB(t *testing.T) *gorm.DB {
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

func TestUserStore_ChangePasswordWithHistory_UpdatesPasswordAndStoresOldHash(t *testing.T) {
	db := setupUserStoreTestDB(t)
	userStore := NewUserStore(db)
	ctx := context.Background()

	user := &model.User{Username: "u1", Email: "u1@example.com", Provider: "local", ProviderID: "u1", InviteCode: "INV-U1"}
	if err := user.SetPassword("Str0ngP@ssword1"); err != nil {
		t.Fatal(err)
	}
	oldHash := user.PasswordHash
	if err := userStore.Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	if err := user.SetPassword("Str0ngP@ssword2"); err != nil {
		t.Fatal(err)
	}
	newHash := user.PasswordHash

	if err := userStore.ChangePasswordWithHistory(ctx, user, oldHash); err != nil {
		t.Fatal(err)
	}

	reloaded, err := userStore.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.PasswordHash != newHash {
		t.Fatalf("expected password hash to be updated to new hash")
	}
	if !reloaded.CheckPassword("Str0ngP@ssword2") {
		t.Fatalf("expected new password to verify")
	}
	if reloaded.CheckPassword("Str0ngP@ssword1") {
		t.Fatalf("expected old password not to verify")
	}

	var histories []model.PasswordHistory
	if err := db.WithContext(ctx).Where("user_id = ?", user.ID).Order("created_at asc").Find(&histories).Error; err != nil {
		t.Fatal(err)
	}
	if len(histories) != 1 {
		t.Fatalf("expected 1 password history record, got %d", len(histories))
	}
	if histories[0].PasswordHash != oldHash {
		t.Fatalf("expected history to store old password hash")
	}
}

func TestUserStore_GetByInviteCode(t *testing.T) {
	db := setupUserStoreTestDB(t)
	userStore := NewUserStore(db)
	ctx := context.Background()

	user := &model.User{Username: "u2", Email: "u2@example.com", Provider: "local", ProviderID: "u2", InviteCode: "INV-CODE-U2"}
	if err := userStore.Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	got, err := userStore.GetByInviteCode(ctx, "INV-CODE-U2")
	if err != nil {
		t.Fatalf("expected user by invite code, got error: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("expected user id %q, got %q", user.ID, got.ID)
	}
}

func TestUserStore_GetByUsernameOrEmail_PrefersUsernameMatch(t *testing.T) {
	db := setupUserStoreTestDB(t)
	userStore := NewUserStore(db)
	ctx := context.Background()

	usernameMatch := &model.User{
		Base:       model.Base{ID: "zzzzzzzz-0000-0000-0000-000000000000"},
		Username:   "identity",
		Email:      "username-match@example.com",
		Provider:   "local",
		ProviderID: "identity-username",
		InviteCode: "INV-IDENTITY-U",
	}
	emailMatch := &model.User{
		Base:       model.Base{ID: "00000000-0000-0000-0000-000000000000"},
		Username:   "another-user",
		Email:      "identity",
		Provider:   "local",
		ProviderID: "identity-email",
		InviteCode: "INV-IDENTITY-E",
	}
	if err := userStore.Create(ctx, usernameMatch); err != nil {
		t.Fatalf("create username match user: %v", err)
	}
	if err := userStore.Create(ctx, emailMatch); err != nil {
		t.Fatalf("create email match user: %v", err)
	}

	got, err := userStore.GetByUsernameOrEmail(ctx, "identity")
	if err != nil {
		t.Fatalf("lookup identity: %v", err)
	}
	if got.ID != usernameMatch.ID {
		t.Fatalf("expected username match %q, got %q", usernameMatch.ID, got.ID)
	}
}
