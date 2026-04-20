package store

import (
	"context"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPasswordHistoryTestDB(t *testing.T) *gorm.DB {
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

func TestPasswordHistoryStore_IsRecentlyUsed(t *testing.T) {
	db := setupPasswordHistoryTestDB(t)
	s := NewPasswordHistoryStore(db)
	ctx := context.Background()

	u := &model.User{Username: "u1", Email: "u1@example.com", Provider: "local", ProviderID: "u1", InviteCode: "INV-U1"}
	if err := u.SetPassword("Str0ngP@ssword1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Create(ctx, &model.PasswordHistory{UserID: u.ID, PasswordHash: u.PasswordHash}); err != nil {
		t.Fatal(err)
	}

	used, err := s.IsRecentlyUsed(ctx, u.ID, "Str0ngP@ssword1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("expected password to be recognized as reused")
	}

	used, err = s.IsRecentlyUsed(ctx, u.ID, "An0therP@ssword2", 5)
	if err != nil {
		t.Fatal(err)
	}
	if used {
		t.Fatal("expected password not to match history")
	}
}

func TestPasswordHistoryStore_TrimByUserID(t *testing.T) {
	db := setupPasswordHistoryTestDB(t)
	s := NewPasswordHistoryStore(db)
	ctx := context.Background()

	u := &model.User{Username: "u2", Email: "u2@example.com", Provider: "local", ProviderID: "u2", InviteCode: "INV-U2"}
	if err := u.SetPassword("Str0ngP@ssword2"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}

	hashes := []string{"h1", "h2", "h3"}
	for _, h := range hashes {
		if err := s.Create(ctx, &model.PasswordHistory{UserID: u.ID, PasswordHash: h}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	if err := s.TrimByUserID(ctx, u.ID, 2); err != nil {
		t.Fatal(err)
	}

	rows, err := s.ListByUserID(ctx, u.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 history rows after trim, got %d", len(rows))
	}
	if rows[0].PasswordHash != "h3" || rows[1].PasswordHash != "h2" {
		t.Fatalf("expected latest hashes [h3 h2], got [%s %s]", rows[0].PasswordHash, rows[1].PasswordHash)
	}
}
