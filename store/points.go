package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ysicing/go-template/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PointStore struct {
	db *gorm.DB

	lbMu      sync.RWMutex
	lbCache   []LeaderboardEntry
	lbCacheAt time.Time
}

func NewPointStore(db *gorm.DB) *PointStore {
	return &PointStore{db: db}
}

// forUpdate applies SELECT FOR UPDATE only on databases that support it (not SQLite).
func forUpdate(tx *gorm.DB) *gorm.DB {
	if tx.Dialector.Name() == "sqlite" {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

// GetOrCreateUserPoints returns the user's point record, creating one if needed.
func (s *PointStore) GetOrCreateUserPoints(ctx context.Context, userID string) (*model.UserPoints, error) {
	var up model.UserPoints
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&up).Error
	if err == nil {
		return &up, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	up = model.UserPoints{UserID: userID, Level: 1}
	if err := s.db.WithContext(ctx).Create(&up).Error; err != nil {
		// Race condition: another goroutine may have created it.
		if err2 := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&up).Error; err2 != nil {
			return nil, err
		}
	}
	return &up, nil
}

// AddPoints adds points to a user within a transaction.
func (s *PointStore) AddPoints(ctx context.Context, userID, pointType string, amount int64, kind, reason, operatorID string) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.AddPointsWithTx(tx, userID, pointType, amount, kind, reason, operatorID)
	})
}

// AddPointsWithTx adds points using an existing transaction handle.
func (s *PointStore) AddPointsWithTx(tx *gorm.DB, userID, pointType string, amount int64, kind, reason, operatorID string) error {
	var up model.UserPoints
	if err := forUpdate(tx).Where("user_id = ?", userID).First(&up).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			up = model.UserPoints{UserID: userID, Level: 1}
			if err := tx.Create(&up).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	}

	switch pointType {
	case model.PointTypePaid:
		up.PaidBalance += amount
	case model.PointTypeFree:
		up.FreeBalance += amount
	default:
		return fmt.Errorf("invalid point type: %s", pointType)
	}
	if model.CountsTowardEXP(kind) {
		up.TotalEarned += amount
	}
	up.Level = model.CalcLevel(up.TotalEarned)

	if err := tx.Save(&up).Error; err != nil {
		return err
	}

	txn := model.PointTransaction{
		UserID:     userID,
		PointType:  pointType,
		Kind:       kind,
		Amount:     amount,
		Balance:    up.TotalBalance(),
		Reason:     reason,
		OperatorID: operatorID,
	}
	return tx.Create(&txn).Error
}

// SpendPoints deducts points (free first, then paid). Does not reduce TotalEarned.
func (s *PointStore) SpendPoints(ctx context.Context, userID string, amount int64, kind, reason string) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var up model.UserPoints
		if err := forUpdate(tx).Where("user_id = ?", userID).First(&up).Error; err != nil {
			return fmt.Errorf("user points not found")
		}

		if up.TotalBalance() < amount {
			return fmt.Errorf("insufficient balance")
		}

		remaining := amount
		usedFree := int64(0)
		usedPaid := int64(0)
		// Deduct from free balance first.
		if up.FreeBalance >= remaining {
			up.FreeBalance -= remaining
			usedFree = remaining
			remaining = 0
		} else {
			usedFree = up.FreeBalance
			remaining -= up.FreeBalance
			up.FreeBalance = 0
		}
		// Deduct remainder from paid balance.
		if remaining > 0 {
			up.PaidBalance -= remaining
			usedPaid = remaining
		}

		if err := tx.Save(&up).Error; err != nil {
			return err
		}

		// Determine point type based on actual deduction source.
		pointType := model.PointTypeFree
		if usedPaid > 0 && usedFree > 0 {
			pointType = model.PointTypeMixed
		} else if usedPaid > 0 {
			pointType = model.PointTypePaid
		}

		txn := model.PointTransaction{
			UserID:    userID,
			PointType: pointType,
			Kind:      kind,
			Amount:    -amount,
			Balance:   up.TotalBalance(),
			Reason:    reason,
		}
		return tx.Create(&txn).Error
	})
}

// AdminAdjust adjusts points for a user (can be positive or negative).
func (s *PointStore) AdminAdjust(ctx context.Context, userID, pointType string, amount int64, reason, operatorID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var up model.UserPoints
		if err := forUpdate(tx).Where("user_id = ?", userID).First(&up).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				up = model.UserPoints{UserID: userID, Level: 1}
				if err := tx.Create(&up).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		switch pointType {
		case model.PointTypePaid:
			up.PaidBalance += amount
		case model.PointTypeFree:
			up.FreeBalance += amount
		default:
			return fmt.Errorf("invalid point type: %s", pointType)
		}

		if up.PaidBalance < 0 || up.FreeBalance < 0 {
			return fmt.Errorf("adjustment would result in negative balance")
		}

		up.Level = model.CalcLevel(up.TotalEarned)

		if err := tx.Save(&up).Error; err != nil {
			return err
		}

		txn := model.PointTransaction{
			UserID:     userID,
			PointType:  pointType,
			Kind:       model.PointKindAdminAdjust,
			Amount:     amount,
			Balance:    up.TotalBalance(),
			Reason:     reason,
			OperatorID: operatorID,
		}
		return tx.Create(&txn).Error
	})
}

// ListTransactions returns paginated transactions for a user.
func (s *PointStore) ListTransactions(ctx context.Context, userID string, page, pageSize int) ([]model.PointTransaction, int64, error) {
	var txns []model.PointTransaction
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	db := s.db.WithContext(ctx)
	if err := db.Model(&model.PointTransaction{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := db.Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&txns).Error; err != nil {
		return nil, 0, err
	}
	return txns, total, nil
}

// ListAllTransactions returns paginated transactions for all users (admin).
func (s *PointStore) ListAllTransactions(ctx context.Context, page, pageSize int) ([]model.PointTransaction, int64, error) {
	var txns []model.PointTransaction
	var total int64

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	db := s.db.WithContext(ctx)
	if err := db.Model(&model.PointTransaction{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := db.Model(&model.PointTransaction{}).Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&txns).Error; err != nil {
		return nil, 0, err
	}
	return txns, total, nil
}

// LeaderboardEntry represents a row in the leaderboard.
type LeaderboardEntry struct {
	UserID      string `json:"user_id"`
	TotalEarned int64  `json:"total_earned"`
	Level       int    `json:"level"`
}

const leaderboardCacheTTL = 30 * time.Second

// GetLeaderboard returns the top users by TotalEarned, cached for 30 seconds.
func (s *PointStore) GetLeaderboard(ctx context.Context, limit int) ([]LeaderboardEntry, error) {
	if limit < 1 {
		limit = 10
	}

	s.lbMu.RLock()
	if s.lbCache != nil && time.Since(s.lbCacheAt) < leaderboardCacheTTL && len(s.lbCache) >= limit {
		result := make([]LeaderboardEntry, min(limit, len(s.lbCache)))
		copy(result, s.lbCache[:len(result)])
		s.lbMu.RUnlock()
		return result, nil
	}
	s.lbMu.RUnlock()

	var entries []LeaderboardEntry
	err := s.db.WithContext(ctx).Model(&model.UserPoints{}).
		Select("user_id, total_earned, level").
		Order("total_earned DESC").
		Limit(limit).
		Find(&entries).Error
	if err != nil {
		return nil, err
	}

	s.lbMu.Lock()
	s.lbCache = entries
	s.lbCacheAt = time.Now()
	s.lbMu.Unlock()

	return entries, nil
}
