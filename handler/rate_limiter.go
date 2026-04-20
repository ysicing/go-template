package handler

import (
	"fmt"
	"time"

	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
)

// RateLimiterConfig defines configuration for the rate limiter middleware.
type RateLimiterConfig struct {
	// Max number of requests allowed within the Expiration window
	Max int

	// Expiration is the time window for rate limiting
	Expiration time.Duration

	// KeyGenerator generates the cache key for rate limiting.
	// If not provided, defaults to using the client IP address.
	KeyGenerator func(c fiber.Ctx) string

	// Cache is the cache implementation used for storing rate limit counters.
	// Supports both MemoryCache (single instance) and Redis (distributed).
	Cache store.Cache
}

// RateLimiter creates a rate limiting middleware using the provided cache.
// This implementation works correctly in multi-replica deployments when using Redis,
// unlike Fiber's built-in limiter which uses in-memory storage.
//
// Example usage:
//
//	authLimiter := RateLimiter(RateLimiterConfig{
//	    Max:        20,
//	    Expiration: 1 * time.Minute,
//	    KeyGenerator: func(c fiber.Ctx) string {
//	        return "auth:" + c.IP()
//	    },
//	    Cache: cache,
//	})
func RateLimiter(config RateLimiterConfig) fiber.Handler {
	// Set defaults
	if config.Max <= 0 {
		config.Max = 10
	}
	if config.Expiration <= 0 {
		config.Expiration = 1 * time.Minute
	}
	if config.KeyGenerator == nil {
		config.KeyGenerator = func(c fiber.Ctx) string {
			return GetRealIPForRateLimit(c, "ratelimit")
		}
	}
	if config.Cache == nil {
		panic("RateLimiter: Cache is required")
	}

	return func(c fiber.Ctx) error {
		// Generate rate limit key
		key := config.KeyGenerator(c)

		// Increment counter atomically
		count, err := config.Cache.Incr(c.Context(), key, config.Expiration)
		if err != nil {
			// On cache error, fail closed to prevent brute-force during outages.
			logger.L.Error().Err(err).Str("key", key).Msg("rate limiter: cache error, failing closed")
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "service temporarily unavailable, please retry",
			})
		}

		// Check if limit exceeded
		if count > int64(config.Max) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
			})
		}

		// Set rate limit headers for client visibility
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", config.Max))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, int64(config.Max)-count)))
		c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(config.Expiration).Unix()))

		return c.Next()
	}
}
