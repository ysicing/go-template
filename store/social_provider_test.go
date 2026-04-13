package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/crypto"

	crand "crypto/rand"
)

type failingReader struct{}

func (failingReader) Read(_ []byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

func setupSocialTestDB(t *testing.T) *gorm.DB {
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

func TestSocialProviderStore_Upsert_Create(t *testing.T) {
	db := setupSocialTestDB(t)
	s := NewSocialProviderStore(db, "")

	p := &model.SocialProvider{
		Name:         "github",
		ClientID:     "cid",
		ClientSecret: "csecret",
		RedirectURL:  "http://localhost/cb",
		Enabled:      true,
	}
	if err := s.Upsert(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetByName(context.Background(), "github")
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientID != "cid" {
		t.Errorf("expected ClientID 'cid', got %q", got.ClientID)
	}
	if !got.Enabled {
		t.Error("expected Enabled to be true")
	}
}

func TestSocialProviderStore_Upsert_Update(t *testing.T) {
	db := setupSocialTestDB(t)
	s := NewSocialProviderStore(db, "")

	p := &model.SocialProvider{
		Name:         "github",
		ClientID:     "cid1",
		ClientSecret: "csecret1",
		Enabled:      true,
	}
	if err := s.Upsert(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	p2 := &model.SocialProvider{
		Name:         "github",
		ClientID:     "cid2",
		ClientSecret: "csecret2",
		Enabled:      false,
	}
	if err := s.Upsert(context.Background(), p2); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetByName(context.Background(), "github")
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientID != "cid2" {
		t.Errorf("expected ClientID 'cid2', got %q", got.ClientID)
	}
	if got.Enabled {
		t.Error("expected Enabled to be false after update")
	}
}

func TestSocialProviderStore_List(t *testing.T) {
	db := setupSocialTestDB(t)
	s := NewSocialProviderStore(db, "")

	s.Upsert(context.Background(), &model.SocialProvider{Name: "github", ClientID: "a", ClientSecret: "b", Enabled: true})
	s.Upsert(context.Background(), &model.SocialProvider{Name: "google", ClientID: "c", ClientSecret: "d", Enabled: true})

	providers, err := s.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestSocialProviderStore_Delete(t *testing.T) {
	db := setupSocialTestDB(t)
	s := NewSocialProviderStore(db, "")

	s.Upsert(context.Background(), &model.SocialProvider{Name: "github", ClientID: "a", ClientSecret: "b", Enabled: true})

	got, _ := s.GetByName(context.Background(), "github")
	if err := s.Delete(context.Background(), got.ID); err != nil {
		t.Fatal(err)
	}

	_, err := s.GetByName(context.Background(), "github")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestSocialProviderStore_GetByID(t *testing.T) {
	db := setupSocialTestDB(t)
	s := NewSocialProviderStore(db, "")

	s.Upsert(context.Background(), &model.SocialProvider{Name: "github", ClientID: "a", ClientSecret: "b", Enabled: true})

	byName, _ := s.GetByName(context.Background(), "github")
	byID, err := s.GetByID(context.Background(), byName.ID)
	if err != nil {
		t.Fatal(err)
	}
	if byID.Name != "github" {
		t.Errorf("expected name 'github', got %q", byID.Name)
	}
}

func TestSocialProviderStore_GetByName_NotFound(t *testing.T) {
	db := setupSocialTestDB(t)
	s := NewSocialProviderStore(db, "")

	_, err := s.GetByName(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestSocialProviderStore_EncryptionRoundTrip(t *testing.T) {
	db := setupSocialTestDB(t)
	encKey := "test-encryption-key"
	s := NewSocialProviderStore(db, encKey)
	ctx := context.Background()

	secret := "super-secret-client-value"
	p := &model.SocialProvider{
		Name: "encrypted-github", ClientID: "cid",
		ClientSecret: secret, RedirectURL: "http://localhost/cb", Enabled: true,
	}
	if err := s.Upsert(ctx, p); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Read back via store — should be decrypted.
	got, err := s.GetByName(ctx, "encrypted-github")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ClientSecret != secret {
		t.Fatalf("expected decrypted secret %q, got %q", secret, got.ClientSecret)
	}

	// Read raw from DB — should be encrypted (has enc: prefix).
	var raw model.SocialProvider
	db.Where("name = ?", "encrypted-github").First(&raw)
	if !strings.HasPrefix(raw.ClientSecret, crypto.EncPrefix) {
		t.Fatalf("expected encrypted value in DB, got %q", raw.ClientSecret)
	}
}

func TestSocialProviderStore_GetByNameReturnsErrorForPlaintextLegacyValue(t *testing.T) {
	db := setupSocialTestDB(t)
	ctx := context.Background()

	// Write plaintext directly (simulating legacy data).
	legacy := &model.SocialProvider{
		Name: "legacy-google", ClientID: "cid",
		ClientSecret: "plain-secret", RedirectURL: "http://localhost/cb", Enabled: true,
	}
	db.Create(legacy)

	// Read with encryption enabled — plaintext legacy secrets are no longer accepted.
	s := NewSocialProviderStore(db, "some-key")
	_, err := s.GetByName(ctx, "legacy-google")
	if !errors.Is(err, ErrSocialProviderSecretUnavailable) {
		t.Fatalf("expected ErrSocialProviderSecretUnavailable, got %v", err)
	}
}

func TestSocialProviderStore_GetByNameReturnsErrorWhenDecryptFails(t *testing.T) {
	db := setupSocialTestDB(t)
	ctx := context.Background()

	writer := NewSocialProviderStore(db, "writer-key")
	if err := writer.Upsert(ctx, &model.SocialProvider{
		Name:         "github",
		ClientID:     "cid",
		ClientSecret: "top-secret",
		RedirectURL:  "https://example.com/callback",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("seed encrypted provider: %v", err)
	}

	reader := NewSocialProviderStore(db, "wrong-reader-key")
	_, err := reader.GetByName(ctx, "github")
	if !errors.Is(err, ErrSocialProviderSecretUnavailable) {
		t.Fatalf("expected ErrSocialProviderSecretUnavailable, got %v", err)
	}
}

func TestSocialProviderStore_UpsertFailsWhenSecretEncryptionFails(t *testing.T) {
	db := setupSocialTestDB(t)
	s := NewSocialProviderStore(db, "encryption-key")
	ctx := context.Background()

	originalReader := crand.Reader
	crand.Reader = failingReader{}
	t.Cleanup(func() { crand.Reader = originalReader })

	err := s.Upsert(ctx, &model.SocialProvider{
		Name:         "broken-github",
		ClientID:     "cid",
		ClientSecret: "super-secret",
		RedirectURL:  "http://localhost/cb",
		Enabled:      true,
	})
	if err == nil {
		t.Fatal("expected encryption failure to be returned")
	}

	var count int64
	if db.WithContext(ctx).Model(&model.SocialProvider{}).Where("name = ?", "broken-github").Count(&count).Error != nil {
		t.Fatal("failed to count rows")
	}
	if count != 0 {
		t.Fatal("expected provider not to be persisted when encryption fails")
	}
}
