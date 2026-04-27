package points

import "github.com/ysicing/go-template/model"

type checkInPointAward struct {
	PointType string
	Amount    int64
	Kind      string
	Reason    string
}

type checkInOutcome struct {
	StreakDays    int
	PointsAwarded int64
	PointAwards   []checkInPointAward
}

func buildCheckInOutcome(previousStreak int, dailyPoints int64) checkInOutcome {
	streakDays := previousStreak + 1
	outcome := checkInOutcome{
		StreakDays:    streakDays,
		PointsAwarded: dailyPoints,
		PointAwards: []checkInPointAward{{
			PointType: model.PointTypeFree,
			Amount:    dailyPoints,
			Kind:      model.PointKindCheckIn,
			Reason:    "daily check-in",
		}},
	}

	if streakDays%model.StreakBonusDays == 0 {
		outcome.PointsAwarded += int64(model.StreakBonusPoints)
		outcome.PointAwards = append(outcome.PointAwards, checkInPointAward{
			PointType: model.PointTypeFree,
			Amount:    int64(model.StreakBonusPoints),
			Kind:      model.PointKindStreakBonus,
			Reason:    "streak bonus",
		})
	}

	return outcome
}

func buildCheckInRecord(userID, checkInDate string, outcome checkInOutcome) model.CheckInRecord {
	return model.CheckInRecord{
		UserID:        userID,
		CheckInDate:   checkInDate,
		StreakDays:    outcome.StreakDays,
		PointsAwarded: outcome.PointsAwarded,
	}
}
