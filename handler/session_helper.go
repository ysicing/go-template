package handler

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
)

func issueBrowserSession(c fiber.Ctx, sessions *service.SessionService, user *model.User, refreshTTL time.Duration) (*service.IssuedSession, error) {
	ip, ua := GetRealIPAndUA(c)
	return sessions.IssueBrowserSession(c.Context(), service.SessionRequest{
		User:       user,
		IP:         ip,
		UserAgent:  ua,
		RefreshTTL: refreshTTL,
	})
}
