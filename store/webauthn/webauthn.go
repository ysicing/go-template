package webauthnstore

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ysicing/go-template/model"
	rootstore "github.com/ysicing/go-template/store"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

// WebAuthnStore handles persistence for WebAuthn credentials.
type WebAuthnStore struct {
	db *gorm.DB
}

// NewWebAuthnStore creates a WebAuthnStore.
func NewWebAuthnStore(db *gorm.DB) *WebAuthnStore {
	return &WebAuthnStore{db: db}
}

// Create stores a new WebAuthn credential.
func (s *WebAuthnStore) Create(ctx context.Context, cred *model.WebAuthnCredential) error {
	return s.db.WithContext(ctx).Create(cred).Error
}

// ListByUserID returns all credentials for a user.
func (s *WebAuthnStore) ListByUserID(ctx context.Context, userID string) ([]model.WebAuthnCredential, error) {
	var creds []model.WebAuthnCredential
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&creds).Error
	return creds, err
}

// GetByID returns a credential by ID.
func (s *WebAuthnStore) GetByID(ctx context.Context, id string) (*model.WebAuthnCredential, error) {
	var cred model.WebAuthnCredential
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&cred).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, rootstore.ErrNotFound
		}
		return nil, err
	}
	return &cred, nil
}

// Delete removes a credential by ID and user ID.
func (s *WebAuthnStore) Delete(ctx context.Context, id, userID string) error {
	return s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.WebAuthnCredential{}).Error
}

// DeleteByUserID removes all credentials for a user.
func (s *WebAuthnStore) DeleteByUserID(ctx context.Context, userID string) error {
	return s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.WebAuthnCredential{}).Error
}

// UpdateSignCount updates the sign count for a credential.
func (s *WebAuthnStore) UpdateSignCount(ctx context.Context, credentialID []byte, signCount uint32) error {
	return s.db.WithContext(ctx).Model(&model.WebAuthnCredential{}).
		Where("credential_id = ?", credentialID).
		Update("sign_count", signCount).Error
}

// WebAuthnUser implements the webauthn.User interface.
type WebAuthnUser struct {
	User  *model.User
	Creds []model.WebAuthnCredential
}

func (u *WebAuthnUser) WebAuthnID() []byte                         { return []byte(u.User.ID) }
func (u *WebAuthnUser) WebAuthnName() string                       { return u.User.Username }
func (u *WebAuthnUser) WebAuthnDisplayName() string                { return u.User.Username }
func (u *WebAuthnUser) WebAuthnIcon() string                       { return u.User.AvatarURL }
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential { return toWebAuthnCreds(u.Creds) }

func toWebAuthnCreds(creds []model.WebAuthnCredential) []webauthn.Credential {
	result := make([]webauthn.Credential, len(creds))
	for i, c := range creds {
		var transports []protocol.AuthenticatorTransport
		if c.Transport != "" {
			_ = json.Unmarshal([]byte(c.Transport), &transports)
		}
		result[i] = webauthn.Credential{
			ID:              c.CredentialID,
			PublicKey:       c.PublicKey,
			AttestationType: "",
			Transport:       transports,
			Flags: webauthn.CredentialFlags{
				BackupEligible: c.BackupEligible,
				BackupState:    c.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:    c.AAGUID,
				SignCount: c.SignCount,
			},
		}
	}
	return result
}
