package points

import (
	"context"
	"testing"

	"github.com/ysicing/go-template/model"
)

func TestPointStoreAddPoints(t *testing.T) {
	db := setupPointsTestDB(t)
	s := NewPointStore(db)
	ctx := context.Background()

	if err := s.AddPoints(ctx, "user-1", model.PointTypeFree, 15, model.PointKindCheckIn, "daily", ""); err != nil {
		t.Fatalf("AddPoints() error = %v", err)
	}

	var up model.UserPoints
	if err := db.WithContext(ctx).Where("user_id = ?", "user-1").First(&up).Error; err != nil {
		t.Fatalf("load user points: %v", err)
	}
	if up.FreeBalance != 15 || up.PaidBalance != 0 {
		t.Fatalf("unexpected balances: free=%d paid=%d", up.FreeBalance, up.PaidBalance)
	}
	if up.TotalEarned != 15 {
		t.Fatalf("unexpected total earned: %d", up.TotalEarned)
	}
	if up.Level != model.CalcLevel(15) {
		t.Fatalf("unexpected level: %d", up.Level)
	}

	var txn model.PointTransaction
	if err := db.WithContext(ctx).Where("user_id = ?", "user-1").First(&txn).Error; err != nil {
		t.Fatalf("load point transaction: %v", err)
	}
	if txn.PointType != model.PointTypeFree || txn.Amount != 15 || txn.Kind != model.PointKindCheckIn {
		t.Fatalf("unexpected txn: %+v", txn)
	}
}

func TestPointStoreSpendPoints_FreeFirstThenPaid(t *testing.T) {
	db := setupPointsTestDB(t)
	s := NewPointStore(db)
	ctx := context.Background()

	if err := s.AdminAdjust(ctx, "user-1", model.PointTypeFree, 5, "seed free", "admin"); err != nil {
		t.Fatalf("seed free points: %v", err)
	}
	if err := s.AdminAdjust(ctx, "user-1", model.PointTypePaid, 4, "seed paid", "admin"); err != nil {
		t.Fatalf("seed paid points: %v", err)
	}
	if err := s.SpendPoints(ctx, "user-1", 7, model.PointKindSpend, "buy"); err != nil {
		t.Fatalf("SpendPoints() error = %v", err)
	}

	var up model.UserPoints
	if err := db.WithContext(ctx).Where("user_id = ?", "user-1").First(&up).Error; err != nil {
		t.Fatalf("load user points: %v", err)
	}
	if up.FreeBalance != 0 || up.PaidBalance != 2 {
		t.Fatalf("unexpected balances after spend: free=%d paid=%d", up.FreeBalance, up.PaidBalance)
	}
	if up.TotalEarned != 0 {
		t.Fatalf("spend should not change total earned, got %d", up.TotalEarned)
	}

	var spendTxn model.PointTransaction
	if err := db.WithContext(ctx).Where("user_id = ? AND kind = ?", "user-1", model.PointKindSpend).First(&spendTxn).Error; err != nil {
		t.Fatalf("load spend transaction: %v", err)
	}
	if spendTxn.PointType != model.PointTypeMixed || spendTxn.Amount != -7 || spendTxn.Balance != 2 {
		t.Fatalf("unexpected spend txn: %+v", spendTxn)
	}
}

func TestPointStoreAdminAdjust_RejectsNegativeBalance(t *testing.T) {
	db := setupPointsTestDB(t)
	s := NewPointStore(db)
	ctx := context.Background()

	if err := s.AdminAdjust(ctx, "user-1", model.PointTypePaid, 3, "seed paid", "admin"); err != nil {
		t.Fatalf("seed paid points: %v", err)
	}
	err := s.AdminAdjust(ctx, "user-1", model.PointTypePaid, -5, "overdraft", "admin")
	if err == nil || err.Error() != "adjustment would result in negative balance" {
		t.Fatalf("expected overdraft error, got %v", err)
	}

	var count int64
	if err := db.WithContext(ctx).Model(&model.PointTransaction{}).Where("user_id = ?", "user-1").Count(&count).Error; err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only seed transaction to persist, got %d", count)
	}
}

func TestPointStoreSpendPoints_UserNotFound(t *testing.T) {
	db := setupPointsTestDB(t)
	s := NewPointStore(db)

	err := s.SpendPoints(context.Background(), "missing-user", 1, model.PointKindSpend, "buy")
	if err == nil || err.Error() != "user points not found" {
		t.Fatalf("expected missing user error, got %v", err)
	}
}

func TestApplyPointCredit_InvalidType(t *testing.T) {
	_, err := applyPointCredit(model.UserPoints{}, "invalid", 1, model.PointKindCheckIn)
	if err == nil || err.Error() != "invalid point type: invalid" {
		t.Fatalf("expected invalid type error, got %v", err)
	}
}

func TestApplyPointSpend_InsufficientBalance(t *testing.T) {
	_, err := applyPointSpend(model.UserPoints{FreeBalance: 1}, 2)
	if err == nil || err.Error() != "insufficient balance" {
		t.Fatalf("expected insufficient balance error, got %v", err)
	}
}

func TestApplyPointAdjustment_InvalidType(t *testing.T) {
	_, err := applyPointAdjustment(model.UserPoints{}, "invalid", 1)
	if err == nil || err.Error() != "invalid point type: invalid" {
		t.Fatalf("expected invalid type error, got %v", err)
	}
}
