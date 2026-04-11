package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
)

const checkInTimezone = "Asia/Shanghai"

var nowForCheckIn = time.Now

func currentCheckInTime() time.Time {
	return nowForCheckIn().In(checkInLocation)
}

var checkInLocation = loadCheckInLocation()

func loadCheckInLocation() *time.Location {
	loc, err := time.LoadLocation(checkInTimezone)
	if err != nil {
		// Fallback keeps the business date boundary at UTC+8 if timezone data is missing.
		return time.FixedZone("CST", 8*60*60)
	}
	return loc
}

func NowInCheckInLocation() time.Time {
	return currentCheckInTime()
}

type CheckInStore struct {
	db     *gorm.DB
	points *PointStore
}

func NewCheckInStore(db *gorm.DB, points *PointStore) *CheckInStore {
	return &CheckInStore{db: db, points: points}
}

// CheckIn performs a daily check-in for the user.
// Returns the record, whether it was newly created, and any error.
func (s *CheckInStore) CheckIn(ctx context.Context, userID string) (*model.CheckInRecord, bool, error) {
	today := todayDateStr()

	// Quick non-transactional check to avoid starting a transaction for repeat check-ins.
	var existing model.CheckInRecord
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND check_in_date = ?", userID, today).
		First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	var record model.CheckInRecord
	isNew := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Re-check inside transaction to prevent race conditions.
		var dup model.CheckInRecord
		if err := tx.Where("user_id = ? AND check_in_date = ?", userID, today).First(&dup).Error; err == nil {
			record = dup
			return nil
		}

		// Calculate streak.
		streak := 1
		yesterday := yesterdayDateStr()
		var yesterdayRecord model.CheckInRecord
		if err := tx.Where("user_id = ? AND check_in_date = ?", userID, yesterday).First(&yesterdayRecord).Error; err == nil {
			streak = yesterdayRecord.StreakDays + 1
		}

		// Calculate points using normal distribution.
		dailyPts := model.RandomCheckInPoints()
		points := dailyPts
		if streak%model.StreakBonusDays == 0 {
			points += int64(model.StreakBonusPoints)
		}

		record = model.CheckInRecord{
			UserID:        userID,
			CheckInDate:   today,
			StreakDays:    streak,
			PointsAwarded: points,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}

		// Award points within the same transaction.
		if err := s.points.AddPointsWithTx(tx, userID, model.PointTypeFree, dailyPts, model.PointKindCheckIn, "daily check-in", ""); err != nil {
			return err
		}

		if streak%model.StreakBonusDays == 0 {
			if err := s.points.AddPointsWithTx(tx, userID, model.PointTypeFree, int64(model.StreakBonusPoints), model.PointKindStreakBonus, "streak bonus", ""); err != nil {
				return err
			}
		}

		isNew = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &record, isNew, nil
}

// GetStreak returns the current streak days for the user.
func (s *CheckInStore) GetStreak(ctx context.Context, userID string) (int, error) {
	today := todayDateStr()
	var record model.CheckInRecord
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND check_in_date = ?", userID, today).
		First(&record).Error
	if err == nil {
		return record.StreakDays, nil
	}
	// If not checked in today, check yesterday for ongoing streak.
	yesterday := yesterdayDateStr()
	err = s.db.WithContext(ctx).
		Where("user_id = ? AND check_in_date = ?", userID, yesterday).
		First(&record).Error
	if err == nil {
		return record.StreakDays, nil
	}
	return 0, nil
}

// GetTodayRecord returns today's check-in record if it exists.
func (s *CheckInStore) GetTodayRecord(ctx context.Context, userID string) (*model.CheckInRecord, error) {
	today := todayDateStr()
	var record model.CheckInRecord
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND check_in_date = ?", userID, today).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// GetMonthlyRecords returns all check-in records for a given month.
func (s *CheckInStore) GetMonthlyRecords(ctx context.Context, userID string, year, month int) ([]model.CheckInRecord, error) {
	startDate := fmt.Sprintf("%04d-%02d-01", year, month)
	// Calculate end date (first day of next month).
	endYear, endMonth := year, month+1
	if endMonth > 12 {
		endMonth = 1
		endYear++
	}
	endDate := fmt.Sprintf("%04d-%02d-01", endYear, endMonth)

	var records []model.CheckInRecord
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND check_in_date >= ? AND check_in_date < ?", userID, startDate, endDate).
		Order("check_in_date ASC").
		Find(&records).Error
	return records, err
}

func todayDateStr() string {
	return currentCheckInTime().Format("2006-01-02")
}

func yesterdayDateStr() string {
	return currentCheckInTime().AddDate(0, 0, -1).Format("2006-01-02")
}
