package points

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	handlercommon "github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/model"
	rootstore "github.com/ysicing/go-template/store"
	pointstore "github.com/ysicing/go-template/store/points"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

func TestPointsHandler_CheckIn_WritesAuditLog(t *testing.T) {
	db := setupTestDB(t)
	points := pointstore.NewPointStore(db)
	checkins := pointstore.NewCheckInStore(db, points)
	audit := rootstore.NewAuditLogStore(db)
	user := createLocalUser(t, db, "points-user", "points-user@example.com", "Password123!abcd")

	h := NewPointsHandler(points, checkins, audit)
	app := fiber.New()
	app.Use(handlercommon.RequestIDMiddleware())
	app.Use(handlercommon.AuditContextMiddleware())
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return c.Next()
	})
	app.Post("/api/points/checkin", h.CheckIn)

	req := httptest.NewRequest(http.MethodPost, "/api/points/checkin", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	assertPointsAuditLogByAction(t, db, user.ID, model.AuditPointsCheckIn, "points")
}

func assertPointsAuditLogByAction(t *testing.T, db *gorm.DB, userID, action, resource string) {
	t.Helper()

	var auditLog model.AuditLog
	if err := db.WithContext(context.Background()).
		Where("user_id = ? AND action = ? AND resource = ?", userID, action, resource).
		Order("created_at DESC").
		First(&auditLog).Error; err != nil {
		t.Fatalf("expected audit log for action %s: %v", action, err)
	}
}
