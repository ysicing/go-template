package points

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
	up, err := s.loadOrCreateUserPointsTx(tx, userID)
	if err != nil {
		return err
	}
	up, err = applyPointCredit(up, pointType, amount, kind)
	if err != nil {
		return err
	}
	if err := tx.Save(&up).Error; err != nil {
		return err
	}

	return tx.Create(&model.PointTransaction{
		UserID:     userID,
		PointType:  pointType,
		Kind:       kind,
		Amount:     amount,
		Balance:    up.TotalBalance(),
		Reason:     reason,
		OperatorID: operatorID,
	}).Error
}

// SpendPoints deducts points (free first, then paid). Does not reduce TotalEarned.
func (s *PointStore) SpendPoints(ctx context.Context, userID string, amount int64, kind, reason string) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		up, err := s.loadUserPointsForUpdate(tx, userID)
		if err != nil {
			return fmt.Errorf("user points not found")
		}
		result, err := applyPointSpend(up, amount)
		if err != nil {
			return err
		}
		if err := tx.Save(&result.UserPoints).Error; err != nil {
			return err
		}
		return tx.Create(&model.PointTransaction{
			UserID:    userID,
			PointType: result.PointType,
			Kind:      kind,
			Amount:    -amount,
			Balance:   result.UserPoints.TotalBalance(),
			Reason:    reason,
		}).Error
	})
}

// AdminAdjust adjusts points for a user (can be positive or negative).
func (s *PointStore) AdminAdjust(ctx context.Context, userID, pointType string, amount int64, reason, operatorID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		up, err := s.loadOrCreateUserPointsTx(tx, userID)
		if err != nil {
			return err
		}
		up, err = applyPointAdjustment(up, pointType, amount)
		if err != nil {
			return err
		}
		if err := tx.Save(&up).Error; err != nil {
			return err
		}
		return tx.Create(&model.PointTransaction{
			UserID:     userID,
			PointType:  pointType,
			Kind:       model.PointKindAdminAdjust,
			Amount:     amount,
			Balance:    up.TotalBalance(),
			Reason:     reason,
			OperatorID: operatorID,
		}).Error
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

func (s *PointStore) loadOrCreateUserPointsTx(tx *gorm.DB, userID string) (model.UserPoints, error) {
	up, err := s.loadUserPointsForUpdate(tx, userID)
	if err == nil {
		return up, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.UserPoints{}, err
	}

	up = model.UserPoints{UserID: userID, Level: 1}
	if err := tx.Create(&up).Error; err != nil {
		return model.UserPoints{}, err
	}
	return up, nil
}

func (s *PointStore) loadUserPointsForUpdate(tx *gorm.DB, userID string) (model.UserPoints, error) {
	var up model.UserPoints
	if err := forUpdate(tx).Where("user_id = ?", userID).First(&up).Error; err != nil {
		return model.UserPoints{}, err
	}
	return up, nil
}
