package cookie

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestSetTokenCookies(t *testing.T) {
	app := fiber.New()
	app.Get("/set", func(c fiber.Ctx) error {
		SetTokenCookies(c, "access-value", "refresh-value", time.Minute, time.Hour)
		return c.SendStatus(fiber.StatusNoContent)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/set", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	cookies := resp.Header.Values("Set-Cookie")
	assertCookieContains(t, cookies, "access_token=access-value", "path=/", "max-age=60", "HttpOnly", "SameSite=Lax")
	assertCookieContains(t, cookies, "refresh_token=refresh-value", "path=/api/auth/", "max-age=3600", "HttpOnly", "SameSite=Lax")
}

func TestClearTokenCookies(t *testing.T) {
	app := fiber.New()
	app.Get("/clear", func(c fiber.Ctx) error {
		ClearTokenCookies(c)
		return c.SendStatus(fiber.StatusNoContent)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/clear", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	cookies := resp.Header.Values("Set-Cookie")
	assertCookieContains(t, cookies, "access_token=", "path=/", "max-age=0", "HttpOnly")
	assertCookieContains(t, cookies, "refresh_token=", "path=/api/auth/", "max-age=0", "HttpOnly")
}

func assertCookieContains(t *testing.T, cookies []string, name string, parts ...string) {
	t.Helper()

	for _, cookie := range cookies {
		if !strings.Contains(cookie, name) {
			continue
		}
		for _, part := range parts {
			if !strings.Contains(cookie, part) {
				t.Fatalf("expected cookie %q to contain %q", cookie, part)
			}
		}
		return
	}
	t.Fatalf("expected Set-Cookie containing %q in %#v", name, cookies)
}
