package httpserver

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/shared"
)

const (
	localUserID = "user_id"
	localRole   = "role"
)

func requireAuth(tokens *auth.TokenManager) fiber.Handler {
	return func(c fiber.Ctx) error {
		if tokens == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(shared.Err("AUTH_UNAVAILABLE", "auth unavailable"))
		}

		header := strings.TrimSpace(c.Get("Authorization"))
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(shared.Err("UNAUTHORIZED", "unauthorized"))
		}

		claims, err := tokens.ParseAccess(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(shared.Err("UNAUTHORIZED", "unauthorized"))
		}

		c.Locals(localUserID, claims.UserID)
		c.Locals(localRole, claims.Role)
		return c.Next()
	}
}

func requireAdmin(c fiber.Ctx) error {
	role, _ := c.Locals(localRole).(string)
	if role != "admin" {
		return c.Status(fiber.StatusForbidden).JSON(shared.Err("FORBIDDEN", "forbidden"))
	}
	return c.Next()
}
