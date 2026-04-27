package points

import (
	"fmt"

	"github.com/ysicing/go-template/model"
)

type spendPointResult struct {
	UserPoints model.UserPoints
	PointType  string
}

func applyPointCredit(up model.UserPoints, pointType string, amount int64, kind string) (model.UserPoints, error) {
	if amount <= 0 {
		return model.UserPoints{}, fmt.Errorf("amount must be positive")
	}
	if err := addPointBalance(&up, pointType, amount); err != nil {
		return model.UserPoints{}, err
	}
	if model.CountsTowardEXP(kind) {
		up.TotalEarned += amount
	}
	up.Level = model.CalcLevel(up.TotalEarned)
	return up, nil
}

func applyPointSpend(up model.UserPoints, amount int64) (spendPointResult, error) {
	if amount <= 0 {
		return spendPointResult{}, fmt.Errorf("amount must be positive")
	}
	if up.TotalBalance() < amount {
		return spendPointResult{}, fmt.Errorf("insufficient balance")
	}

	remaining := amount
	usedFree := minInt64(up.FreeBalance, remaining)
	up.FreeBalance -= usedFree
	remaining -= usedFree

	usedPaid := remaining
	up.PaidBalance -= usedPaid

	return spendPointResult{
		UserPoints: up,
		PointType:  spendPointType(usedFree, usedPaid),
	}, nil
}

func applyPointAdjustment(up model.UserPoints, pointType string, amount int64) (model.UserPoints, error) {
	if err := addPointBalance(&up, pointType, amount); err != nil {
		return model.UserPoints{}, err
	}
	if up.PaidBalance < 0 || up.FreeBalance < 0 {
		return model.UserPoints{}, fmt.Errorf("adjustment would result in negative balance")
	}
	up.Level = model.CalcLevel(up.TotalEarned)
	return up, nil
}

func addPointBalance(up *model.UserPoints, pointType string, amount int64) error {
	switch pointType {
	case model.PointTypePaid:
		up.PaidBalance += amount
	case model.PointTypeFree:
		up.FreeBalance += amount
	default:
		return fmt.Errorf("invalid point type: %s", pointType)
	}
	return nil
}

func spendPointType(usedFree, usedPaid int64) string {
	switch {
	case usedPaid > 0 && usedFree > 0:
		return model.PointTypeMixed
	case usedPaid > 0:
		return model.PointTypePaid
	default:
		return model.PointTypeFree
	}
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
