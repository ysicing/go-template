package model

import "golang.org/x/crypto/bcrypt"

// PasswordHistory stores previous password hashes to prevent reuse.
type PasswordHistory struct {
	Base
	UserID       string `gorm:"index;type:varchar(36)"`
	PasswordHash string `gorm:"type:varchar(255)"`
}

func (PasswordHistory) TableName() string { return "password_histories" }

// MatchesPassword checks if the given plaintext password matches this history entry.
func (ph *PasswordHistory) MatchesPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(ph.PasswordHash), []byte(password)) == nil
}
