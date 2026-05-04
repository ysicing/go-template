package cookie

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

// SetTokenCookies 将访问令牌和刷新令牌写入 HttpOnly Cookie。
func SetTokenCookies(c fiber.Ctx, accessToken, refreshToken string, accessTTL, refreshTTL time.Duration) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		MaxAge:   int(accessTTL.Seconds()),
		HTTPOnly: true,
		Secure:   c.Scheme() == "https",
		SameSite: "Lax",
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/api/auth/",
		MaxAge:   int(refreshTTL.Seconds()),
		HTTPOnly: true,
		Secure:   c.Scheme() == "https",
		SameSite: "Lax",
	})
}

// ClearTokenCookies 清除认证 Cookie，通常用于退出登录。
func ClearTokenCookies(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth/",
		MaxAge:   -1,
		HTTPOnly: true,
	})
}
