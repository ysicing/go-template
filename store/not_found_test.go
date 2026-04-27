package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ysicing/go-template/model"

	"gorm.io/gorm"
)

func TestNormalizeNotFound(t *testing.T) {
	if !errors.Is(normalizeNotFound(gorm.ErrRecordNotFound), ErrNotFound) {
		t.Fatalf("expected gorm.ErrRecordNotFound to normalize to ErrNotFound")
	}
	if !errors.Is(normalizeNotFound(ErrNotFound), ErrNotFound) {
		t.Fatalf("expected ErrNotFound to remain ErrNotFound")
	}
}

func TestStoresNormalizeNotFound(t *testing.T) {
	db := setupUserStoreTestDB(t)
	ctx := context.Background()

	userStore := NewUserStore(db)
	if _, err := userStore.GetByID(ctx, "missing-user"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected user store ErrNotFound, got %v", err)
	}

	clientStore := NewOAuthClientStore(db)
	if _, err := clientStore.GetByClientID(ctx, "missing-client"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected oauth client store ErrNotFound, got %v", err)
	}

	grantStore := NewOAuthConsentGrantStore(db)
	if _, err := grantStore.GetByUserAndClient(ctx, "missing-user", "missing-client"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected consent grant store ErrNotFound, got %v", err)
	}
	if err := grantStore.DeleteByIDAndUserID(ctx, "missing-grant", "missing-user"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected consent grant delete ErrNotFound, got %v", err)
	}

	socialStore := NewSocialAccountStore(db)
	if _, err := socialStore.GetByProviderAndID(ctx, model.SocialProviderGitHub, "missing-provider-id"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected social account store ErrNotFound, got %v", err)
	}
	if _, err := socialStore.GetByProviderForUser(ctx, "missing-user", model.SocialProviderGitHub); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected social account-by-user store ErrNotFound, got %v", err)
	}

	providerStore := NewSocialProviderStore(db, "")
	if _, err := providerStore.GetByName(ctx, "missing-provider"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected social provider store ErrNotFound, got %v", err)
	}

	mfaStore := NewMFAStore(db, "")
	if _, err := mfaStore.GetByUserID(ctx, "missing-user"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected mfa store ErrNotFound, got %v", err)
	}

	refreshStore := NewAPIRefreshTokenStore(db)
	if _, err := refreshStore.GetByTokenHash(ctx, HashToken("missing-token")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected refresh token store ErrNotFound, got %v", err)
	}
	if err := refreshStore.DeleteByIDAndUserID(ctx, "missing-token-id", "missing-user"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected refresh token delete ErrNotFound, got %v", err)
	}
}
