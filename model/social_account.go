package model

const (
	SocialProviderGitHub = "github"
	SocialProviderGoogle = "google"
)

// SocialAccount represents a social login provider linked to a user account.
type SocialAccount struct {
	Base
	UserID      string `gorm:"type:varchar(36);not null;index:idx_user_provider" json:"user_id"`
	Provider    string `gorm:"type:varchar(50);not null;index:idx_user_provider;index:idx_provider_id" json:"provider"` // github, google, etc.
	ProviderID  string `gorm:"type:varchar(255);not null;index:idx_provider_id" json:"provider_id"`                     // unique ID from provider
	Username    string `gorm:"type:varchar(255)" json:"username"`
	DisplayName string `gorm:"type:varchar(255)" json:"display_name"`
	Email       string `gorm:"type:varchar(255)" json:"email"` // email from provider
	AvatarURL   string `gorm:"type:varchar(500)" json:"avatar_url"`

	User *User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName specifies the table name for SocialAccount.
func (SocialAccount) TableName() string {
	return "social_accounts"
}
