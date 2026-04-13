package app

import (
	"net/url"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	fiberrecover "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	fibersession "github.com/gofiber/fiber/v3/middleware/session"
	"github.com/rs/zerolog"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"
)

func setupMiddlewareChain(app *fiber.App, settingStore *store.SettingStore, sessionStorage fiber.Storage, log *zerolog.Logger) {
	app.Use(fiberrecover.New(fiberrecover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, err any) {
			logger.L.Error().
				Str("request_id", requestid.FromContext(c)).
				Str("trace_id", store.TraceIDFromContext(c.Context())).
				Str("method", c.Method()).
				Str("path", c.Path()).
				Str("stack", string(debug.Stack())).
				Interface("panic", err).
				Msg("panic recovered")
		},
	}))
	app.Use(requestid.New())
	app.Use(fibersession.New(fibersession.Config{
		Storage:         sessionStorage,
		CookieHTTPOnly:  true,
		CookieSameSite:  "Lax",
		IdleTimeout:     30 * time.Minute,
		AbsoluteTimeout: 24 * time.Hour,
		Next: func(c fiber.Ctx) bool {
			return shouldSkipSession(c)
		},
	}))
	app.Use(traceContextMiddleware())
	app.Use(cookieCSRFMiddleware())
	app.Use(requestLogMiddleware())
	app.Use(handler.PrometheusMiddleware())
	app.Use(buildCORSMiddleware(settingStore, log))
	app.Use(compress.New())
	app.Use(securityHeadersMiddleware(settingStore))
	app.Use(handler.AuditContextMiddleware())
}

func shouldSkipSession(c fiber.Ctx) bool {
	path := c.Path()
	if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
		return true
	}
	return path == "/health" ||
		strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/assets/") ||
		strings.HasPrefix(path, "/swagger/")
}

func traceContextMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		trace := store.TraceContext{
			RequestID: requestid.FromContext(c),
			TraceID:   resolveTraceID(c.Get("X-Trace-ID"), c.Get("Traceparent"), requestid.FromContext(c)),
			SpanID:    store.NewSpanID(),
		}
		if sess := fibersession.FromContext(c); sess != nil && sess.Session != nil {
			trace.SessionID = sess.Session.ID()
		}
		c.Locals("request_id", trace.RequestID)
		c.Locals("trace_id", trace.TraceID)
		c.Locals("span_id", trace.SpanID)
		if trace.SessionID != "" {
			c.Locals("session_id", trace.SessionID)
		}
		c.Set("X-Trace-ID", trace.TraceID)
		c.SetContext(store.WithTraceContext(c.Context(), trace))
		return c.Next()
	}
}

func cookieCSRFMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if isSafeMethod(c.Method()) || !hasAuthCookies(c) {
			return c.Next()
		}
		if isSameOriginRequest(c) {
			return c.Next()
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "csrf validation failed"})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return true
	default:
		return false
	}
}

func hasAuthCookies(c fiber.Ctx) bool {
	return c.Cookies("access_token") != "" || c.Cookies("refresh_token") != ""
}

func isSameOriginRequest(c fiber.Ctx) bool {
	expectedScheme := strings.ToLower(c.Scheme())
	expectedHost := strings.ToLower(c.Get("Host"))
	if expectedHost == "" {
		expectedHost = strings.ToLower(c.Hostname())
	}
	for _, raw := range []string{c.Get("Origin"), c.Get("Referer")} {
		if raw == "" {
			continue
		}
		if matchesOrigin(raw, expectedScheme, expectedHost) {
			return true
		}
		return false
	}
	return false
}

func matchesOrigin(raw, expectedScheme, expectedHost string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if !strings.EqualFold(parsed.Scheme, expectedScheme) {
		return false
	}
	return strings.EqualFold(parsed.Host, expectedHost)
}

func resolveTraceID(headerTraceID, traceparent, requestID string) string {
	if tid := strings.TrimSpace(headerTraceID); tid != "" {
		return tid
	}
	parts := strings.Split(strings.TrimSpace(traceparent), "-")
	if len(parts) == 4 && len(parts[1]) == 32 {
		return strings.ToLower(parts[1])
	}
	traceID := store.NewTraceID()
	if traceID != "" {
		return traceID
	}
	return requestID
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
		trace := store.TraceContextFromContext(c.Context())
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
			Str("ua", ua)
		if trace.RequestID != "" {
			event = event.Str("request_id", trace.RequestID)
		}
		if trace.TraceID != "" {
			event = event.Str("trace_id", trace.TraceID)
		}
		if trace.SpanID != "" {
			event = event.Str("span_id", trace.SpanID)
		}
		if trace.SessionID != "" {
			event = event.Str("session_id", trace.SessionID)
		}
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
