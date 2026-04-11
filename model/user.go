package model

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

const (
	PermissionAdminUsersRead        = "admin.users.read"
	PermissionAdminUsersWrite       = "admin.users.write"
	PermissionAdminClientsRead      = "admin.clients.read"
	PermissionAdminClientsWrite     = "admin.clients.write"
	PermissionAdminProvidersRead    = "admin.providers.read"
	PermissionAdminProvidersWrite   = "admin.providers.write"
	PermissionAdminSettingsRead     = "admin.settings.read"
	PermissionAdminSettingsWrite    = "admin.settings.write"
	PermissionAdminPointsRead       = "admin.points.read"
	PermissionAdminPointsWrite      = "admin.points.write"
	PermissionAdminLoginHistoryRead = "admin.login_history.read"
	PermissionAdminStatsRead        = "admin.stats.read"
)

var allAdminPermissions = []string{
	PermissionAdminUsersRead,
	PermissionAdminUsersWrite,
	PermissionAdminClientsRead,
	PermissionAdminClientsWrite,
	PermissionAdminProvidersRead,
	PermissionAdminProvidersWrite,
	PermissionAdminSettingsRead,
	PermissionAdminSettingsWrite,
	PermissionAdminPointsRead,
	PermissionAdminPointsWrite,
	PermissionAdminLoginHistoryRead,
	PermissionAdminStatsRead,
}

type User struct {
	Base
	Username        string     `gorm:"uniqueIndex;type:varchar(255)" json:"username"`
	Email           string     `gorm:"uniqueIndex;type:varchar(255)" json:"email"`
	PasswordHash    string     `gorm:"type:varchar(255)" json:"-"`
	IsAdmin         bool       `gorm:"default:false" json:"is_admin"`
	Provider        string     `gorm:"type:varchar(50);default:'local';uniqueIndex:idx_provider_providerid" json:"provider"`
	ProviderID      string     `gorm:"type:varchar(255);uniqueIndex:idx_provider_providerid" json:"-"`
	AvatarURL       string     `gorm:"type:varchar(512)" json:"avatar_url,omitempty"`
	EmailVerified   bool       `gorm:"default:false" json:"email_verified"`
	EmailUpdatedAt  *time.Time `json:"email_updated_at,omitempty"`
	InviteCode      string     `gorm:"uniqueIndex;type:varchar(32)" json:"invite_code,omitempty"`
	InvitedByUserID string     `gorm:"index;type:varchar(36)" json:"invited_by_user_id,omitempty"`
	InviteIP        string     `gorm:"type:varchar(45)" json:"invite_ip,omitempty"`
	Permissions     string     `gorm:"type:varchar(1024)" json:"permissions,omitempty"`
	TokenVersion    int64      `gorm:"type:bigint;not null;default:1" json:"-"`
}

func (User) TableName() string { return "users" }

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

func (u User) PermissionList() []string {
	if u.Permissions == "" {
		if u.IsAdmin {
			return append([]string(nil), allAdminPermissions...)
		}
		return nil
	}
	parts := strings.Split(u.Permissions, ",")
	perms := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		perm := strings.TrimSpace(part)
		if perm == "" {
			continue
		}
		if _, ok := seen[perm]; ok {
			continue
		}
		seen[perm] = struct{}{}
		perms = append(perms, perm)
	}
	slices.Sort(perms)
	return perms
}

func (u User) HasPermission(permission string) bool {
	if permission == "" {
		return true
	}
	perms := u.PermissionList()
	for _, perm := range perms {
		if perm == permission {
			return true
		}
	}
	return false
}

func (u *User) SetPermissions(perms []string) {
	if len(perms) == 0 {
		u.Permissions = ""
		return
	}
	cleaned := make([]string, 0, len(perms))
	seen := make(map[string]struct{}, len(perms))
	for _, perm := range perms {
		p := strings.TrimSpace(perm)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		cleaned = append(cleaned, p)
	}
	if len(cleaned) == 0 {
		u.Permissions = ""
		return
	}
	slices.Sort(cleaned)
	u.Permissions = strings.Join(cleaned, ",")
}

func IsValidPermission(permission string) bool {
	for _, perm := range allAdminPermissions {
		if perm == permission {
			return true
		}
	}
	return false
}

func AllAdminPermissions() []string {
	return append([]string(nil), allAdminPermissions...)
}

// ValidatePasswordStrength checks password meets policy requirements:
// min 12 chars, must contain upper, lower, digit, special char.
func ValidatePasswordStrength(password string) error {
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}
	return nil
}
