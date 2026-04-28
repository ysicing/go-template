package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

func TestUserHandler_GetMe_IncludesPermissionList(t *testing.T) {
	db := setupTestDB(t)
	userStore := store.NewUserStore(db)
	user := createLocalUser(t, db, "perm-user", "perm-user@example.com", "Password123!abcd")
	user.SetPermissions([]string{model.PermissionAdminStatsRead})
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save permissions: %v", err)
	}

	h := NewUserHandler(UserDeps{Users: userStore})
	app := fiber.New()
	app.Get("/api/users/me", func(c fiber.Ctx) error {
		c.Locals("user_id", user.ID)
		return h.GetMe(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body struct {
		User struct {
			ID          string   `json:"id"`
			Permissions []string `json:"permissions"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.User.ID != user.ID {
		t.Fatalf("expected user id %q, got %q", user.ID, body.User.ID)
	}
	if len(body.User.Permissions) != 1 || body.User.Permissions[0] != model.PermissionAdminStatsRead {
		t.Fatalf("expected permissions [%s], got %#v", model.PermissionAdminStatsRead, body.User.Permissions)
	}
}
