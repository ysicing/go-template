package store

import (
	"context"
	"time"

	"github.com/ysicing/go-template/model"

	"gorm.io/gorm"
)

// AuditLogStore handles persistence for audit logs.
type AuditLogStore struct {
	db *gorm.DB
}

// NewAuditLogStore creates an AuditLogStore.
func NewAuditLogStore(db *gorm.DB) *AuditLogStore {
	return &AuditLogStore{db: db}
}

// Create stores a new audit log entry.
func (s *AuditLogStore) Create(ctx context.Context, log *model.AuditLog) error {
	return s.db.WithContext(ctx).Create(log).Error
}

// ListByUser returns audit logs for a specific user.
func (s *AuditLogStore) ListByUser(ctx context.Context, userID string, page, pageSize int) ([]model.AuditLog, int64, error) {
	var logs []model.AuditLog
	var total int64
	q := s.db.WithContext(ctx).Model(&model.AuditLog{}).Where("user_id = ?", userID)
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

// --- Login statistics (replaces LoginEventStore) ---

// CountLogin returns the total number of login events.
func (s *AuditLogStore) CountLogin(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.AuditLog{}).
		Where("action = ?", model.AuditLogin).Count(&count).Error
	return count, err
}

// CountLoginToday returns today's login count.
func (s *AuditLogStore) CountLoginToday(ctx context.Context) (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := s.db.WithContext(ctx).Model(&model.AuditLog{}).
		Where("action = ? AND created_at >= ?", model.AuditLogin, today).Count(&count).Error
	return count, err
}

// CountLoginByUserID returns total login count for a specific user.
func (s *AuditLogStore) CountLoginByUserID(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.AuditLog{}).
		Where("user_id = ? AND action = ?", userID, model.AuditLogin).Count(&count).Error
	return count, err
}
