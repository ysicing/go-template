package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_MemoryCache(t *testing.T) {
	cache := store.NewMemoryCache()
	defer cache.Close()

	app := fiber.New()
	app.Get("/test", RateLimiter(RateLimiterConfig{
		Max:        3,
		Expiration: 1 * time.Second,
		KeyGenerator: func(c fiber.Ctx) string {
			return GetRealIPForRateLimit(c, "test")
		},
		Cache: cache,
	}), func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
	resp.Body.Close()

	// Wait for expiration
	time.Sleep(1100 * time.Millisecond)

	// Should work again after expiration
	req2 := httptest.NewRequest("GET", "/test", nil)
	resp2, err := app.Test(req2)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp2.StatusCode)
	resp2.Body.Close()
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	cache := store.NewMemoryCache()
	defer cache.Close()

	app := fiber.New()

	// Use custom key generator for testing
	app.Get("/test", RateLimiter(RateLimiterConfig{
		Max:        2,
		Expiration: 1 * time.Second,
		KeyGenerator: func(c fiber.Ctx) string {
			// Alternate between two keys to simulate different IPs
			key := c.Query("key", "default")
			return "test:" + key
		},
		Cache: cache,
	}), func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Key "a" - 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test?key=a", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// Key "b" - should still work (different key)
	req := httptest.NewRequest("GET", "/test?key=b", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Key "a" again - should be rate limited (3rd request)
	req2 := httptest.NewRequest("GET", "/test?key=a", nil)
	resp2, err := app.Test(req2)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusTooManyRequests, resp2.StatusCode)
	resp2.Body.Close()
}

func TestRateLimiter_RemainingHeaderNeverNegative(t *testing.T) {
	cache := store.NewMemoryCache()
	defer cache.Close()

	app := fiber.New()
	app.Get("/test", RateLimiter(RateLimiterConfig{
		Max:        1,
		Expiration: 1 * time.Second,
		Cache:      cache,
	}), func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	req1 := httptest.NewRequest("GET", "/test", nil)
	resp1, err := app.Test(req1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp1.StatusCode)
	assert.Equal(t, "0", resp1.Header.Get("X-RateLimit-Remaining"))
	resp1.Body.Close()

	req2 := httptest.NewRequest("GET", "/test", nil)
	resp2, err := app.Test(req2)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusTooManyRequests, resp2.StatusCode)
	resp2.Body.Close()
}

func TestRateLimiter_CacheError_FailClosed(t *testing.T) {
	// Create a mock cache that always returns errors
	mockCache := &mockErrorCache{}

	app := fiber.New()
	app.Get("/test", RateLimiter(RateLimiterConfig{
		Max:        1,
		Expiration: 1 * time.Second,
		Cache:      mockCache,
	}), func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Should return 503 when cache errors (fail closed)
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
	resp.Body.Close()
}

// mockErrorCache always returns errors
type mockErrorCache struct{}

func (m *mockErrorCache) Ping(ctx context.Context) error { return nil }
func (m *mockErrorCache) Close() error                   { return nil }
func (m *mockErrorCache) Get(ctx context.Context, key string) (string, error) {
	return "", store.ErrCacheMiss
}
func (m *mockErrorCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return nil
}
func (m *mockErrorCache) Del(ctx context.Context, key string) error { return nil }
func (m *mockErrorCache) DelIfValue(ctx context.Context, key, value string) (bool, error) {
	return false, nil
}
func (m *mockErrorCache) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return false, nil
}
func (m *mockErrorCache) RefreshIfValue(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return false, nil
}
func (m *mockErrorCache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return 0, assert.AnError
}
func (m *mockErrorCache) GetInt(ctx context.Context, key string) (int64, error) {
	return 0, store.ErrCacheMiss
}
