package store

import (
	"context"

	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/crypto"
)

// SocialProviderStore handles persistence for social login provider configurations.
type SocialProviderStore struct {
	db            *gorm.DB
	encPassphrase string // Passphrase for encrypting client secrets
}

// NewSocialProviderStore creates a SocialProviderStore.
// If encryptionKey is non-empty, client_secret will be encrypted at rest.
func NewSocialProviderStore(db *gorm.DB, encryptionKey string) *SocialProviderStore {
	return &SocialProviderStore{db: db, encPassphrase: encryptionKey}
}

func (s *SocialProviderStore) encryptSecret(plaintext string) (string, error) {
	if s.encPassphrase == "" || plaintext == "" {
		return plaintext, nil
	}
	enc, err := crypto.Encrypt(s.encPassphrase, plaintext)
	if err != nil {
		return "", err
	}
	return enc, nil
}

func (s *SocialProviderStore) decryptSecret(stored string) string {
	if s.encPassphrase == "" || stored == "" {
		return stored
	}
	dec, err := crypto.DecryptOrPlaintext(s.encPassphrase, stored)
	if err != nil {
		return stored // fallback to raw value on error
	}
	return dec
}

func (s *SocialProviderStore) decryptProvider(p *model.SocialProvider) {
	p.ClientSecret = s.decryptSecret(p.ClientSecret)
}

// GetByName retrieves a social provider by name (e.g. "github", "google").
func (s *SocialProviderStore) GetByName(ctx context.Context, name string) (*model.SocialProvider, error) {
	var p model.SocialProvider
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&p).Error; err != nil {
		return nil, err
	}
	s.decryptProvider(&p)
	return &p, nil
}

// GetByID retrieves a social provider by primary key.
func (s *SocialProviderStore) GetByID(ctx context.Context, id string) (*model.SocialProvider, error) {
	var p model.SocialProvider
	if err := s.db.WithContext(ctx).First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	s.decryptProvider(&p)
	return &p, nil
}

// Upsert creates or updates a social provider by name.
func (s *SocialProviderStore) Upsert(ctx context.Context, provider *model.SocialProvider) error {
	encSecret, err := s.encryptSecret(provider.ClientSecret)
	if err != nil {
		return err
	}
	var existing model.SocialProvider
	d := s.db.WithContext(ctx)
	err = d.Where("name = ?", provider.Name).First(&existing).Error
	if err == nil {
		existing.ClientID = provider.ClientID
		existing.ClientSecret = encSecret
		existing.RedirectURL = provider.RedirectURL
		existing.Enabled = provider.Enabled
		return d.Save(&existing).Error
	}
	provider.ClientSecret = encSecret
	err = d.Create(provider).Error
	// Restore plaintext on the in-memory struct so callers see the original value.
	provider.ClientSecret = s.decryptSecret(encSecret)
	return err
}

// List returns all social providers.
func (s *SocialProviderStore) List(ctx context.Context) ([]model.SocialProvider, error) {
	var providers []model.SocialProvider
	if err := s.db.WithContext(ctx).Find(&providers).Error; err != nil {
		return nil, err
	}
	for i := range providers {
		s.decryptProvider(&providers[i])
	}
	return providers, nil
}

// Delete removes a social provider by primary key.
func (s *SocialProviderStore) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.SocialProvider{}).Error
}
