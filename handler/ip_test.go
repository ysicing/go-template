package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

// trustAll configures the package-level trusted proxies to trust all IPs,
// which is needed in tests that use httptest (remote addr is 0.0.0.0).
// It returns a cleanup function that resets trusted proxies to none.
func trustAll(t *testing.T) func() {
	t.Helper()
	SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	return func() { SetTrustedProxies(nil) }
}

func TestGetRealIP_CFConnectingIP(t *testing.T) {
	defer trustAll(t)()

	app := fiber.New(fiber.Config{
		TrustProxy: true,
	})

	var capturedIP string
	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = GetRealIP(c)
		return c.SendString("OK")
	})

	// CF-Connecting-IP should have highest priority
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("CF-Connecting-IP", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	req.Header.Set("X-Forwarded-For", "9.10.11.12, 13.14.15.16")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "1.2.3.4", capturedIP)
	resp.Body.Close()
}

func TestGetRealIP_XRealIP(t *testing.T) {
	defer trustAll(t)()

	app := fiber.New(fiber.Config{
		TrustProxy: true,
	})

	var capturedIP string
	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = GetRealIP(c)
		return c.SendString("OK")
	})

	// X-Real-IP should be used when CF-Connecting-IP is absent
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "5.6.7.8")
	req.Header.Set("X-Forwarded-For", "9.10.11.12, 13.14.15.16")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "5.6.7.8", capturedIP)
	resp.Body.Close()
}

func TestGetRealIP_XForwardedFor(t *testing.T) {
	defer trustAll(t)()

	app := fiber.New(fiber.Config{
		TrustProxy: true,
	})

	var capturedIP string
	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = GetRealIP(c)
		return c.SendString("OK")
	})

	// X-Forwarded-For should use leftmost (client) IP
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "9.10.11.12, 13.14.15.16, 17.18.19.20")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "9.10.11.12", capturedIP)
	resp.Body.Close()
}

func TestGetRealIP_XForwardedForWithSpaces(t *testing.T) {
	defer trustAll(t)()

	app := fiber.New(fiber.Config{
		TrustProxy: true,
	})

	var capturedIP string
	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = GetRealIP(c)
		return c.SendString("OK")
	})

	// Should handle spaces in X-Forwarded-For
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", " 9.10.11.12 , 13.14.15.16 ")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "9.10.11.12", capturedIP)
	resp.Body.Close()
}

func TestGetRealIP_InvalidIPs(t *testing.T) {
	defer trustAll(t)()

	app := fiber.New(fiber.Config{
		TrustProxy: true,
	})

	var capturedIP string
	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = GetRealIP(c)
		return c.SendString("OK")
	})

	// Should skip invalid IPs and use fallback
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("CF-Connecting-IP", "invalid-ip")
	req.Header.Set("X-Real-IP", "also-invalid")
	req.Header.Set("X-Forwarded-For", "not-an-ip, 5.6.7.8")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	// Should fallback to RemoteAddr (test environment default)
	assert.NotEmpty(t, capturedIP)
	resp.Body.Close()
}

func TestGetRealIPForRateLimit(t *testing.T) {
	defer trustAll(t)()

	app := fiber.New(fiber.Config{
		TrustProxy: true,
	})

	var capturedKey string
	app.Get("/test", func(c fiber.Ctx) error {
		capturedKey = GetRealIPForRateLimit(c, "test:")
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("CF-Connecting-IP", "1.2.3.4")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "test:1.2.3.4", capturedKey)
	resp.Body.Close()
}

// TestGetRealIP_UntrustedProxy verifies that forwarded headers are ignored
// when the direct connection does not come from a configured trusted proxy.
func TestGetRealIP_UntrustedProxy(t *testing.T) {
	// No trusted proxies configured — all forwarded headers must be ignored
	SetTrustedProxies(nil)
	defer SetTrustedProxies(nil)

	app := fiber.New()

	var capturedIP string
	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = GetRealIP(c)
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("CF-Connecting-IP", "1.2.3.4")
	req.Header.Set("X-Real-IP", "5.6.7.8")
	req.Header.Set("X-Forwarded-For", "9.10.11.12")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	// Must NOT return any of the forged header values
	assert.NotEqual(t, "1.2.3.4", capturedIP)
	assert.NotEqual(t, "5.6.7.8", capturedIP)
	assert.NotEqual(t, "9.10.11.12", capturedIP)
	assert.NotEmpty(t, capturedIP)
	resp.Body.Close()
}

// TestSetTrustedProxies verifies CIDR parsing for various input formats.
func TestSetTrustedProxies(t *testing.T) {
	defer SetTrustedProxies(nil)

	tests := []struct {
		name     string
		cidrs    []string
		ip       string
		expected bool
	}{
		{"CIDR range match", []string{"10.0.0.0/8"}, "10.1.2.3", true},
		{"CIDR range no match", []string{"10.0.0.0/8"}, "192.168.1.1", false},
		{"bare IPv4 match", []string{"1.2.3.4"}, "1.2.3.4", true},
		{"bare IPv4 no match", []string{"1.2.3.4"}, "1.2.3.5", false},
		{"empty list", []string{}, "1.2.3.4", false},
		{"nil list", nil, "1.2.3.4", false},
		{"invalid CIDR skipped", []string{"not-a-cidr", "10.0.0.0/8"}, "10.5.5.5", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetTrustedProxies(tc.cidrs)
			got := isTrustedProxy(tc.ip)
			assert.Equal(t, tc.expected, got, "isTrustedProxy(%q) with cidrs %v", tc.ip, tc.cidrs)
		})
	}
}
