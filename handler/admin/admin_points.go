package adminhandler

import (
	"strconv"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
	pointstore "github.com/ysicing/go-template/store/points"

	"github.com/gofiber/fiber/v3"
)

type AdminPointsHandler struct {
	points *pointstore.PointStore
	audit  *store.AuditLogStore
}

func NewAdminPointsHandler(points *pointstore.PointStore, audit *store.AuditLogStore) *AdminPointsHandler {
	return &AdminPointsHandler{points: points, audit: audit}
}

// AdjustPoints adjusts a user's points (admin action).
func (h *AdminPointsHandler) AdjustPoints(c fiber.Ctx) error {
	operatorID, _ := c.Locals("user_id").(string)

	var req struct {
		UserID    string `json:"user_id"`
		PointType string `json:"point_type"`
		Amount    int64  `json:"amount"`
		Reason    string `json:"reason"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.UserID == "" || req.Amount == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id and non-zero amount are required"})
	}
	if req.PointType != model.PointTypePaid && req.PointType != model.PointTypeFree {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "point_type must be 'paid' or 'free'"})
	}

	if err := h.points.AdminAdjust(c.Context(), req.UserID, req.PointType, req.Amount, req.Reason, operatorID); err != nil {
		msg := "failed to adjust points"
		if err.Error() == "adjustment would result in negative balance" {
			msg = "adjustment would result in negative balance"
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
	}

	_ = handlercommon.RecordAuditFromFiber(c, h.audit, handlercommon.AuditEvent{
		UserID:     operatorID,
		Action:     model.AuditPointsAdjust,
		Resource:   "points",
		ResourceID: req.UserID,
		Status:     "success",
		Detail:     "points adjusted",
		Metadata: map[string]string{
			"point_type": req.PointType,
			"amount":     strconv.FormatInt(req.Amount, 10),
			"reason":     req.Reason,
		},
	})

	return c.JSON(fiber.Map{"message": "points adjusted successfully"})
}

// GetUserPoints returns a specific user's points (admin).
func (h *AdminPointsHandler) GetUserPoints(c fiber.Ctx) error {
	userID := c.Params("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}
	up, err := h.points.GetOrCreateUserPoints(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get user points"})
	}
	return c.JSON(fiber.Map{
		"points":        up,
		"total_balance": up.TotalBalance(),
	})
}

// GetAllTransactions returns paginated transactions for all users (admin).
func (h *AdminPointsHandler) GetAllTransactions(c fiber.Ctx) error {
	page, pageSize := handlercommon.ParsePagination(c)
	txns, total, err := h.points.ListAllTransactions(c.Context(), page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list transactions"})
	}
	return c.JSON(fiber.Map{
		"transactions": txns,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// GetLeaderboard returns the top users by points.
func (h *AdminPointsHandler) GetLeaderboard(c fiber.Ctx) error {
	entries, err := h.points.GetLeaderboard(c.Context(), 50)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get leaderboard"})
	}
	return c.JSON(fiber.Map{"leaderboard": entries})
}
