package store

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

func newTestConsentGrantStore(t *testing.T) *OAuthConsentGrantStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return NewOAuthConsentGrantStore(db)
}

func TestOAuthConsentGrantStoreUpsertMergesScopes(t *testing.T) {
	s := newTestConsentGrantStore(t)
	ctx := context.Background()

	if err := s.Upsert(ctx, &model.OAuthConsentGrant{
		UserID:   "user-1",
		ClientID: "client-1",
		Scopes:   "openid profile",
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	if err := s.Upsert(ctx, &model.OAuthConsentGrant{
		UserID:   "user-1",
		ClientID: "client-1",
		Scopes:   "openid email",
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := s.GetByUserAndClient(ctx, "user-1", "client-1")
	if err != nil {
		t.Fatalf("get grant: %v", err)
	}
	if got.Scopes != "email openid profile" {
		t.Fatalf("expected merged scopes, got %q", got.Scopes)
	}
}

func TestOAuthConsentGrantStoreHasGrantedScopes(t *testing.T) {
	s := newTestConsentGrantStore(t)
	ctx := context.Background()

	if err := s.Upsert(ctx, &model.OAuthConsentGrant{
		UserID:   "user-1",
		ClientID: "client-1",
		Scopes:   "openid profile email",
	}); err != nil {
		t.Fatalf("upsert grant: %v", err)
	}

	granted, err := s.HasGrantedScopes(ctx, "user-1", "client-1", []string{"openid", "profile"})
	if err != nil {
		t.Fatalf("check subset scopes: %v", err)
	}
	if !granted {
		t.Fatal("expected subset scopes to be granted")
	}

	granted, err = s.HasGrantedScopes(ctx, "user-1", "client-1", []string{"openid", "admin"})
	if err != nil {
		t.Fatalf("check uncovered scopes: %v", err)
	}
	if granted {
		t.Fatal("expected uncovered scope set to require new consent")
	}
}

func TestOAuthConsentGrantStoreListByUserIDPaged(t *testing.T) {
	s := newTestConsentGrantStore(t)
	ctx := context.Background()

	if err := s.Upsert(ctx, &model.OAuthConsentGrant{UserID: "user-1", ClientID: "client-a", Scopes: "openid"}); err != nil {
		t.Fatalf("seed first grant: %v", err)
	}
	if err := s.Upsert(ctx, &model.OAuthConsentGrant{UserID: "user-1", ClientID: "client-b", Scopes: "openid profile"}); err != nil {
		t.Fatalf("seed second grant: %v", err)
	}
	if err := s.Upsert(ctx, &model.OAuthConsentGrant{UserID: "user-2", ClientID: "client-c", Scopes: "openid"}); err != nil {
		t.Fatalf("seed third grant: %v", err)
	}

	grants, total, err := s.ListByUserIDPaged(ctx, "user-1", 1, 10)
	if err != nil {
		t.Fatalf("list grants: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(grants) != 2 {
		t.Fatalf("expected 2 grants, got %d", len(grants))
	}
}

func TestOAuthConsentGrantStoreDeleteByIDAndUserID(t *testing.T) {
	s := newTestConsentGrantStore(t)
	ctx := context.Background()

	if err := s.Upsert(ctx, &model.OAuthConsentGrant{UserID: "user-1", ClientID: "client-a", Scopes: "openid"}); err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	got, err := s.GetByUserAndClient(ctx, "user-1", "client-a")
	if err != nil {
		t.Fatalf("get grant: %v", err)
	}

	if err := s.DeleteByIDAndUserID(ctx, got.ID, "user-1"); err != nil {
		t.Fatalf("delete grant: %v", err)
	}

	if _, err := s.GetByUserAndClient(ctx, "user-1", "client-a"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected deleted grant to be missing, got %v", err)
	}
}
