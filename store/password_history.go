package store

import (
	"context"

	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

// PasswordHistoryStore handles persistence for password history entries.
type PasswordHistoryStore struct {
	db *gorm.DB
}

// NewPasswordHistoryStore creates a PasswordHistoryStore.
func NewPasswordHistoryStore(db *gorm.DB) *PasswordHistoryStore {
	return &PasswordHistoryStore{db: db}
}

// Create persists a password history entry.
func (s *PasswordHistoryStore) Create(ctx context.Context, entry *model.PasswordHistory) error {
	return s.db.WithContext(ctx).Create(entry).Error
}

// ListByUserID returns latest password history entries for a user.
func (s *PasswordHistoryStore) ListByUserID(ctx context.Context, userID string, limit int) ([]model.PasswordHistory, error) {
	if limit <= 0 {
		limit = 5
	}
	var rows []model.PasswordHistory
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// IsRecentlyUsed checks whether plaintext password matches recent history entries.
func (s *PasswordHistoryStore) IsRecentlyUsed(ctx context.Context, userID, plaintext string, limit int) (bool, error) {
	rows, err := s.ListByUserID(ctx, userID, limit)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.MatchesPassword(plaintext) {
			return true, nil
		}
	}
	return false, nil
}

// TrimByUserID keeps only the newest `keep` history entries for a user.
func (s *PasswordHistoryStore) TrimByUserID(ctx context.Context, userID string, keep int) error {
	if keep < 0 {
		keep = 0
	}

	var staleIDs []string
	if err := s.db.WithContext(ctx).
		Model(&model.PasswordHistory{}).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(keep).
		Pluck("id", &staleIDs).Error; err != nil {
		return err
	}
	if len(staleIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).
		Where("id IN ?", staleIDs).
		Delete(&model.PasswordHistory{}).Error
}
