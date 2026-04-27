package points

import (
	"strconv"
	"strings"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
	rootstore "github.com/ysicing/go-template/store"
	pointstore "github.com/ysicing/go-template/store/points"

	"github.com/gofiber/fiber/v3"
)

const (
	maxSpendAmount  int64 = 10000
	maxReasonLength int   = 200
)

type PointsHandler struct {
	points  *pointstore.PointStore
	checkin *pointstore.CheckInStore
	audit   *rootstore.AuditLogStore
}

func NewPointsHandler(points *pointstore.PointStore, checkin *pointstore.CheckInStore, audit *rootstore.AuditLogStore) *PointsHandler {
	return &PointsHandler{points: points, checkin: checkin, audit: audit}
}

// GetMyPoints returns the current user's points and level.
func (h *PointsHandler) GetMyPoints(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	up, err := h.points.GetOrCreateUserPoints(c.Context(), userID)
	if err != nil {
		logger.L.Error().Err(err).Str("user_id", userID).Msg("failed to get points")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get points"})
	}
	return c.JSON(fiber.Map{
		"points":        up,
		"total_balance": up.TotalBalance(),
	})
}

// GetTransactions returns paginated point transactions for the current user.
func (h *PointsHandler) GetTransactions(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	page, pageSize := handlercommon.ParsePagination(c)
	txns, total, err := h.points.ListTransactions(c.Context(), userID, page, pageSize)
	if err != nil {
		logger.L.Error().Err(err).Str("user_id", userID).Msg("failed to list transactions")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list transactions"})
	}
	return c.JSON(fiber.Map{
		"transactions": txns,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// CheckIn performs a daily check-in.
func (h *PointsHandler) CheckIn(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	record, isNew, err := h.checkin.CheckIn(c.Context(), userID)
	if err != nil {
		logger.L.Error().Err(err).Str("user_id", userID).Msg("check-in failed")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "check-in failed"})
	}
	if !isNew {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "already checked in today", "record": record})
	}

	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
		UserID:   userID,
		Action:   model.AuditPointsCheckIn,
		Resource: "points",
		Status:   "success",
		Detail:   "daily check-in completed",
		Metadata: map[string]string{
			"record_id": record.ID,
		},
	})

	return c.JSON(fiber.Map{
		"record":  record,
		"message": "check-in successful",
	})
}

// GetCheckInStatus returns today's check-in status and monthly calendar.
func (h *PointsHandler) GetCheckInStatus(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)

	todayRecord, err := h.checkin.GetTodayRecord(c.Context(), userID)
	if err != nil {
		logger.L.Error().Err(err).Str("user_id", userID).Msg("failed to get check-in status")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get check-in status"})
	}

	streak, _ := h.checkin.GetStreak(c.Context(), userID)

	now := pointstore.NowInCheckInLocation()
	year, _ := strconv.Atoi(c.Query("year", strconv.Itoa(now.Year())))
	month, _ := strconv.Atoi(c.Query("month", strconv.Itoa(int(now.Month()))))
	if month < 1 || month > 12 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "month must be between 1 and 12"})
	}
	if year < 2000 || year > 2100 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid year"})
	}

	records, err := h.checkin.GetMonthlyRecords(c.Context(), userID, year, month)
	if err != nil {
		logger.L.Error().Err(err).Str("user_id", userID).Msg("failed to get monthly records")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get monthly records"})
	}

	return c.JSON(fiber.Map{
		"checked_in_today": todayRecord != nil,
		"today_record":     todayRecord,
		"streak_days":      streak,
		"monthly_records":  records,
	})
}

// SpendPoints deducts points from the current user.
func (h *PointsHandler) SpendPoints(c fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)

	var req struct {
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "amount must be positive"})
	}
	if req.Amount > maxSpendAmount {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "amount exceeds maximum allowed (10000)"})
	}

	req.Reason = strings.TrimSpace(req.Reason)
	if len(req.Reason) > maxReasonLength {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "reason must be at most 200 characters"})
	}

	if err := h.points.SpendPoints(c.Context(), userID, req.Amount, model.PointKindSpend, req.Reason); err != nil {
		logger.L.Error().Err(err).Str("user_id", userID).Int64("amount", req.Amount).Msg("spend points failed")
		msg := "spend points failed"
		if err.Error() == "insufficient balance" {
			msg = "insufficient balance"
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
	}

	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
		UserID:   userID,
		Action:   model.AuditPointsSpend,
		Resource: "points",
		Status:   "success",
		Detail:   "points spent",
		Metadata: map[string]string{
			"amount": strconv.FormatInt(req.Amount, 10),
			"reason": req.Reason,
		},
	})

	return c.JSON(fiber.Map{"message": "points spent successfully"})
}
