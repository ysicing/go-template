package store

import (
	"context"
	"testing"
	"time"

	"github.com/ysicing/go-template/model"
)

func withFixedCheckInReward(t *testing.T, points int64) {
	t.Helper()
	old := randomCheckInPoints
	t.Cleanup(func() {
		randomCheckInPoints = old
	})
	randomCheckInPoints = func() int64 { return points }
}

func TestCheckIn_CreatesRecordAndAwardsDailyPoints(t *testing.T) {
	db := setupUserStoreTestDB(t)
	points := NewPointStore(db)
	checkins := NewCheckInStore(db, points)
	ctx := context.Background()

	withFixedCheckInTime(t, time.Date(2026, 3, 2, 1, 0, 0, 0, time.UTC))
	withFixedCheckInReward(t, 3)

	record, isNew, err := checkins.CheckIn(ctx, "user-1")
	if err != nil {
		t.Fatalf("CheckIn() error = %v", err)
	}
	if !isNew {
		t.Fatal("expected first check-in to create a new record")
	}
	if record.StreakDays != 1 || record.PointsAwarded != 3 {
		t.Fatalf("unexpected record: %+v", record)
	}

	var up model.UserPoints
	if err := db.WithContext(ctx).Where("user_id = ?", "user-1").First(&up).Error; err != nil {
		t.Fatalf("load user points: %v", err)
	}
	if up.FreeBalance != 3 || up.TotalEarned != 3 {
		t.Fatalf("unexpected user points: %+v", up)
	}

	var txns []model.PointTransaction
	if err := db.WithContext(ctx).Where("user_id = ?", "user-1").Order("created_at ASC").Find(&txns).Error; err != nil {
		t.Fatalf("load point transactions: %v", err)
	}
	if len(txns) != 1 || txns[0].Kind != model.PointKindCheckIn || txns[0].Amount != 3 {
		t.Fatalf("unexpected transactions: %+v", txns)
	}
}

func TestCheckIn_AddsStreakBonusOnSeventhDay(t *testing.T) {
	db := setupUserStoreTestDB(t)
	points := NewPointStore(db)
	checkins := NewCheckInStore(db, points)
	ctx := context.Background()

	withFixedCheckInTime(t, time.Date(2026, 3, 7, 16, 30, 0, 0, time.UTC))
	withFixedCheckInReward(t, 4)

	if err := db.WithContext(ctx).Create(&model.CheckInRecord{
		UserID:        "user-1",
		CheckInDate:   yesterdayDateStr(),
		StreakDays:    6,
		PointsAwarded: 2,
	}).Error; err != nil {
		t.Fatalf("seed yesterday check-in: %v", err)
	}

	record, isNew, err := checkins.CheckIn(ctx, "user-1")
	if err != nil {
		t.Fatalf("CheckIn() error = %v", err)
	}
	if !isNew {
		t.Fatal("expected seventh-day check-in to create a new record")
	}
	wantTotal := int64(4 + model.StreakBonusPoints)
	if record.StreakDays != 7 || record.PointsAwarded != wantTotal {
		t.Fatalf("unexpected record: %+v", record)
	}

	var up model.UserPoints
	if err := db.WithContext(ctx).Where("user_id = ?", "user-1").First(&up).Error; err != nil {
		t.Fatalf("load user points: %v", err)
	}
	if up.FreeBalance != wantTotal || up.TotalEarned != wantTotal {
		t.Fatalf("unexpected user points: %+v", up)
	}

	var txns []model.PointTransaction
	if err := db.WithContext(ctx).Where("user_id = ?", "user-1").Order("created_at ASC").Find(&txns).Error; err != nil {
		t.Fatalf("load point transactions: %v", err)
	}
	if len(txns) != 2 {
		t.Fatalf("expected 2 point transactions, got %d", len(txns))
	}
	if txns[0].Kind != model.PointKindCheckIn || txns[1].Kind != model.PointKindStreakBonus {
		t.Fatalf("unexpected transactions: %+v", txns)
	}
}

func TestGetTodayRecord_ReturnsNilWhenMissing(t *testing.T) {
	db := setupUserStoreTestDB(t)
	checkins := NewCheckInStore(db, NewPointStore(db))

	record, err := checkins.GetTodayRecord(context.Background(), "missing-user")
	if err != nil {
		t.Fatalf("GetTodayRecord() error = %v", err)
	}
	if record != nil {
		t.Fatalf("expected nil record for missing check-in, got %+v", record)
	}
}
