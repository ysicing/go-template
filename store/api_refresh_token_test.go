package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"
)

func newTestAPIRefreshTokenStore(t *testing.T, cache Cache) *APIRefreshTokenStore {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "refresh-token.db")
	db, err := InitDB("sqlite", dbPath, "error")
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	if err := model.Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return NewAPIRefreshTokenStore(db, cache)
}

func TestAPIRefreshTokenStore_ConsumeTokenCachesUsedFamilyUntilExpiry(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache()
	t.Cleanup(func() { _ = cache.Close() })

	store := newTestAPIRefreshTokenStore(t, cache)
	tokenHash := HashToken("refresh-token")
	token := &model.APIRefreshToken{
		UserID:    "user-1",
		TokenHash: tokenHash,
		Family:    "family-1",
		ExpiresAt: time.Now().Add(60 * time.Millisecond),
	}
	if err := store.Create(ctx, token); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	consumed, err := store.ConsumeToken(ctx, tokenHash)
	if err != nil {
		t.Fatalf("ConsumeToken() error = %v", err)
	}
	if consumed.Family != token.Family {
		t.Fatalf("ConsumeToken() family = %q, want %q", consumed.Family, token.Family)
	}

	family, err := store.GetUsedFamily(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetUsedFamily() error = %v", err)
	}
	if family != token.Family {
		t.Fatalf("GetUsedFamily() = %q, want %q", family, token.Family)
	}

	time.Sleep(100 * time.Millisecond)

	if _, err := store.GetUsedFamily(ctx, tokenHash); err != ErrCacheMiss {
		t.Fatalf("GetUsedFamily() after expiry error = %v, want %v", err, ErrCacheMiss)
	}
}
