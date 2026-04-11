package model

// WebAuthnCredential stores a WebAuthn/Passkey credential.
type WebAuthnCredential struct {
	Base
	UserID         string `gorm:"index;type:varchar(36)" json:"user_id"`
	Name           string `gorm:"type:varchar(255)" json:"name"`
	CredentialID   []byte `gorm:"uniqueIndex" json:"-"`
	PublicKey      []byte `json:"-"`
	AAGUID         []byte `json:"-"`
	SignCount      uint32 `json:"sign_count"`
	BackupEligible bool   `json:"backup_eligible"`
	BackupState    bool   `json:"backup_state"`
	Transport      string `gorm:"type:varchar(255)" json:"transport"` // JSON array
}

func (WebAuthnCredential) TableName() string { return "webauthn_credentials" }
