package points

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ysicing/go-template/model"

	"gorm.io/gorm"
)

const checkInTimezone = "Asia/Shanghai"

var nowForCheckIn = time.Now
var randomCheckInPoints = model.RandomCheckInPoints

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
	existing, found, err := s.findRecord(s.db.WithContext(ctx), userID, today)
	if err != nil {
		return nil, false, err
	}
	if found {
		return &existing, false, nil
	}

	var record model.CheckInRecord
	isNew := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		dup, found, err := s.findRecord(tx, userID, today)
		if err != nil {
			return err
		}
		if found {
			record = dup
			return nil
		}

		prevStreak, err := s.loadStreak(tx, userID, yesterdayDateStr())
		if err != nil {
			return err
		}
		outcome := buildCheckInOutcome(prevStreak, randomCheckInPoints())
		record = buildCheckInRecord(userID, today, outcome)
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		for _, award := range outcome.PointAwards {
			if err := s.points.AddPointsWithTx(tx, userID, award.PointType, award.Amount, award.Kind, award.Reason, ""); err != nil {
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

// GetTodayRecord returns today's check-in record.
// 未签到属于状态查询的正常结果，因此未找到时返回 (nil, nil)。
func (s *CheckInStore) GetTodayRecord(ctx context.Context, userID string) (*model.CheckInRecord, error) {
	record, found, err := s.findRecord(s.db.WithContext(ctx), userID, todayDateStr())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
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

func (s *CheckInStore) findRecord(db *gorm.DB, userID, date string) (model.CheckInRecord, bool, error) {
	var record model.CheckInRecord
	err := db.Where("user_id = ? AND check_in_date = ?", userID, date).First(&record).Error
	if err == nil {
		return record, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.CheckInRecord{}, false, nil
	}
	return model.CheckInRecord{}, false, err
}

func (s *CheckInStore) loadStreak(tx *gorm.DB, userID, date string) (int, error) {
	record, found, err := s.findRecord(tx, userID, date)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}
	return record.StreakDays, nil
}
