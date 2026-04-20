package store

import (
	"context"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/crypto"

	"gorm.io/gorm"
)

// MFAStore handles persistence for MFA configurations.
type MFAStore struct {
	db            *gorm.DB
	encPassphrase string // Passphrase for encrypting TOTP secret at rest
}

// NewMFAStore creates a MFAStore.
func NewMFAStore(db *gorm.DB, encryptionKey string) *MFAStore {
	return &MFAStore{db: db, encPassphrase: encryptionKey}
}

// GetByUserID returns the MFA config for a user.
func (s *MFAStore) GetByUserID(ctx context.Context, userID string) (*model.MFAConfig, error) {
	var cfg model.MFAConfig
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&cfg).Error; err != nil {
		return nil, err
	}
	// Decrypt TOTP secret if encrypted.
	if s.encPassphrase != "" && cfg.TOTPSecret != "" {
		decrypted, err := crypto.DecryptOrPlaintext(s.encPassphrase, cfg.TOTPSecret)
		if err != nil {
			return nil, err
		}
		cfg.TOTPSecret = decrypted
	}
	return &cfg, nil
}

// Upsert creates or updates the MFA config for a user.
func (s *MFAStore) Upsert(ctx context.Context, cfg *model.MFAConfig) error {
	// Encrypt TOTP secret before storing.
	if s.encPassphrase != "" && cfg.TOTPSecret != "" && !crypto.IsEncrypted(cfg.TOTPSecret) {
		encrypted, err := crypto.Encrypt(s.encPassphrase, cfg.TOTPSecret)
		if err != nil {
			return err
		}
		cfg.TOTPSecret = encrypted
	}
	var existing model.MFAConfig
	err := s.db.WithContext(ctx).Where("user_id = ?", cfg.UserID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.WithContext(ctx).Create(cfg).Error
	}
	if err != nil {
		return err
	}
	cfg.ID = existing.ID
	return s.db.WithContext(ctx).Save(cfg).Error
}

// Delete removes the MFA config for a user.
func (s *MFAStore) Delete(ctx context.Context, userID string) error {
	return s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.MFAConfig{}).Error
}
