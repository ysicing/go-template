package model

// MFAConfig stores MFA settings for a user.
type MFAConfig struct {
	Base
	UserID      string `gorm:"uniqueIndex;type:varchar(36)" json:"user_id"`
	TOTPSecret  string `gorm:"type:varchar(255)" json:"-"`
	TOTPEnabled bool   `gorm:"default:false" json:"totp_enabled"`
	BackupCodes string `gorm:"type:text" json:"-"` // JSON array of bcrypt hashed codes
}

func (MFAConfig) TableName() string { return "mfa_configs" }
