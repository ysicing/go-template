package app

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/rs/zerolog"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"
)

func setupMiddlewareChain(app *fiber.App, settingStore *store.SettingStore, log *zerolog.Logger) {
	app.Use(buildCORSMiddleware(settingStore, log))
	app.Use(compress.New(), requestid.New())
	app.Use(securityHeadersMiddleware(settingStore))
	app.Use(func(c fiber.Ctx) error {
		ctx := store.WithRequestID(c.Context(), requestid.FromContext(c))
		c.SetContext(ctx)
		return c.Next()
	})
	app.Use(requestLogMiddleware())
}

func securityHeadersMiddleware(settingStore *store.SettingStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Relaxed CSP for monitor endpoint (needs external resources)
		if c.Path() == "/api/admin/monitor" {
			c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https:; connect-src 'self'")
			return c.Next()
		}

		scriptSrc := "'self' 'unsafe-inline'"
		connectSrc := "'self'"
		frameSrc := "'none'" // Default: block all frames
		if settingStore.Get(store.SettingTurnstileSiteKey, "") != "" {
			scriptSrc += " https://challenges.cloudflare.com"
			connectSrc += " https://challenges.cloudflare.com"
			frameSrc = "https://challenges.cloudflare.com" // Allow Turnstile iframe
		}
		c.Set("Content-Security-Policy", "default-src 'self'; script-src "+scriptSrc+"; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src "+connectSrc+"; frame-src "+frameSrc)
		return c.Next()
	}
}

func requestLogMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		ua := c.Get("User-Agent")
		if strings.HasPrefix(ua, "kube-probe/") {
			return err
		}
		uid := "-"
		if u, ok := c.Locals("user_id").(string); ok && u != "" {
			uid = u
		}
		status := c.Response().StatusCode()
		event := logger.L.Info()
		if status >= 500 {
			event = logger.L.Error()
		} else if status >= 400 {
			event = logger.L.Warn()
		}
		event.
			Str("ip", handler.GetRealIP(c)).
			Int("status", status).
			Dur("latency", time.Since(start)).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("uid", uid).
			Str("ua", ua).
			Str("request_id", requestid.FromContext(c))
		event.Msg("request")
		return err
	}
}

func buildCORSMiddleware(settingStore *store.SettingStore, log *zerolog.Logger) fiber.Handler {
	var (
		corsWarnOnce     sync.Once
		wildcardWarnOnce sync.Once
	)

	allowOriginsFunc := func(origin string) bool {
		if origin == "" {
			return true
		}

		origins := settingStore.GetStringSlice(store.SettingCORSOrigins, nil)
		if len(origins) == 0 {
			corsWarnOnce.Do(func() {
				log.Warn().Msg("CORS: no origins configured, denying cross-origin requests")
			})
			return false
		}

		for _, o := range origins {
			if o == "*" || o == origin {
				return true
			}
		}
		return false
	}

	withCred := cors.New(cors.Config{
		AllowOriginsFunc: allowOriginsFunc,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	})
	withoutCred := cors.New(cors.Config{
		AllowOriginsFunc: allowOriginsFunc,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: false,
	})

	return func(c fiber.Ctx) error {
		origins := settingStore.GetStringSlice(store.SettingCORSOrigins, nil)
		if slices.Contains(origins, "*") {
			wildcardWarnOnce.Do(func() {
				log.Warn().Msg("CORS: wildcard origin detected, credentials disabled for security")
			})
			return withoutCred(c)
		}
		return withCred(c)
	}
}
