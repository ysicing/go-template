package model

import (
	"math"
	"math/rand/v2"
)

// Level thresholds based on total earned EXP.
var levelThresholds = []struct {
	Level     int
	MinPoints int64
}{
	{6, 15000},
	{5, 6000},
	{4, 2400},
	{3, 800},
	{2, 200},
	{1, 0},
}

// CalcLevel returns the level for the given totalEarned points.
func CalcLevel(totalEarned int64) int {
	for _, t := range levelThresholds {
		if totalEarned >= t.MinPoints {
			return t.Level
		}
	}
	return 1
}

// CountsTowardEXP reports whether a positive points gain should increase EXP.
// Currency top-ups and manual admin adjustments are excluded to avoid fast level boosting.
func CountsTowardEXP(kind string) bool {
	switch kind {
	case PointKindCheckIn, PointKindStreakBonus, PointKindInviteReward:
		return true
	default:
		return false
	}
}

// Point type constants.
const (
	PointTypePaid  = "paid"
	PointTypeFree  = "free"
	PointTypeMixed = "mixed"
)

// Point transaction kind constants.
const (
	PointKindCheckIn      = "checkin"
	PointKindStreakBonus  = "streak_bonus"
	PointKindAdminAdjust  = "admin_adjust"
	PointKindSpend        = "spend"
	PointKindRefund       = "refund"
	PointKindPurchase     = "purchase"
	PointKindInviteReward = "invite_reward"
)

// Check-in reward constants.
const (
	StreakBonusDays   = 7
	StreakBonusPoints = 20
)

// RandomCheckInPoints returns a random check-in reward in [1, 5] using a
// normal distribution (mean=3.0, stddev=1.0), clamped and rounded.
// Minimum is 1 to ensure AddPoints (which requires amount > 0) always succeeds.
func RandomCheckInPoints() int64 {
	const mean, stddev, minPts, maxPts = 3.0, 1.0, 1, 5
	v := rand.NormFloat64()*stddev + mean
	v = math.Round(v)
	if v < minPts {
		v = minPts
	}
	if v > maxPts {
		v = maxPts
	}
	return int64(v)
}

func RandomInviteRewardPoints() int64 {
	return rand.Int64N(5) + 1
}

// UserPoints holds a user's point balances and level.
type UserPoints struct {
	Base
	UserID      string `gorm:"uniqueIndex;type:varchar(36);not null" json:"user_id"`
	PaidBalance int64  `gorm:"default:0;not null" json:"paid_balance"`
	FreeBalance int64  `gorm:"default:0;not null" json:"free_balance"`
	TotalEarned int64  `gorm:"default:0;not null" json:"total_earned"`
	Level       int    `gorm:"default:1;not null" json:"level"`
}

func (UserPoints) TableName() string { return "user_points" }

// TotalBalance returns the sum of paid and free balances.
func (p *UserPoints) TotalBalance() int64 {
	return p.PaidBalance + p.FreeBalance
}

// PointTransaction records a single point change.
type PointTransaction struct {
	Base
	UserID     string `gorm:"index;type:varchar(36);not null" json:"user_id"`
	PointType  string `gorm:"type:varchar(10);not null" json:"point_type"`
	Kind       string `gorm:"type:varchar(20);not null;index" json:"kind"`
	Amount     int64  `gorm:"not null" json:"amount"`
	Balance    int64  `gorm:"not null" json:"balance"`
	Reason     string `gorm:"type:varchar(255)" json:"reason"`
	OperatorID string `gorm:"type:varchar(36)" json:"operator_id"`
}

func (PointTransaction) TableName() string { return "point_transactions" }

// CheckInRecord stores daily check-in data.
type CheckInRecord struct {
	Base
	UserID        string `gorm:"type:varchar(36);not null;uniqueIndex:idx_user_date" json:"user_id"`
	CheckInDate   string `gorm:"type:varchar(10);not null;uniqueIndex:idx_user_date" json:"check_in_date"`
	StreakDays    int    `gorm:"default:1;not null" json:"streak_days"`
	PointsAwarded int64  `gorm:"default:0;not null" json:"points_awarded"`
}

func (CheckInRecord) TableName() string { return "check_in_records" }
