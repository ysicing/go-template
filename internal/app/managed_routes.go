package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/store"
)

type managedRouteSpec struct {
	Doc      openAPIRoute
	Handlers func(managedRouteRuntime) []fiber.Handler
}

type managedRouteRuntime struct {
	deps              *Deps
	handlers          *builtHandlers
	jwtMW             fiber.Handler
	tokenVersionMW    fiber.Handler
	emailVerified     fiber.Handler
	optionalJWT       fiber.Handler
	authLimiter       fiber.Handler
	turnstile         fiber.Handler
	ghLimiter         fiber.Handler
	pointsLimiter     fiber.Handler
	registerLimiter   fiber.Handler
	registerEnabledMW fiber.Handler
	authConfigHandler fiber.Handler
}

func buildManagedRouteRuntime(d *Deps, h *builtHandlers) managedRouteRuntime {
	cfg := d.Config
	return managedRouteRuntime{
		deps:              d,
		handlers:          h,
		jwtMW:             handler.JWTMiddleware(cfg.JWT.Secret, cfg.JWT.Issuer),
		tokenVersionMW:    handler.TokenVersionMiddleware(d.UserStore, d.Cache),
		emailVerified:     handler.EmailVerifiedMiddleware(d.UserStore, d.SettingStore, d.Cache),
		optionalJWT:       handler.OptionalJWTMiddleware(cfg.JWT.Secret, cfg.JWT.Issuer),
		authLimiter:       newAuthLimiter(d.Cache),
		turnstile:         turnstileMiddleware(d.SettingStore),
		ghLimiter:         newGitHubCompatLimiter(d.Cache),
		pointsLimiter:     newPointsLimiter(d.Cache),
		registerLimiter:   newRegisterLimiter(d.Cache),
		registerEnabledMW: registerEnabledMiddleware(d.SettingStore),
		authConfigHandler: authConfigHandler(d.SettingStore),
	}
}

func newAuthLimiter(cache store.Cache) fiber.Handler {
	return handler.RateLimiter(handler.RateLimiterConfig{
		Max: 20, Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string { return handler.GetRealIPForRateLimit(c, "auth:") },
		Cache:        cache,
	})
}

func newPointsLimiter(cache store.Cache) fiber.Handler {
	return handler.RateLimiter(handler.RateLimiterConfig{
		Max: 10, Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			if uid, _ := c.Locals("user_id").(string); uid != "" {
				return "points:" + uid
			}
			return handler.GetRealIPForRateLimit(c, "points:")
		},
		Cache: cache,
	})
}

func newGitHubCompatLimiter(cache store.Cache) fiber.Handler {
	return handler.RateLimiter(handler.RateLimiterConfig{
		Max: 120, Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string { return handler.GetRealIPForRateLimit(c, "gh_compat:") },
		Cache:        cache,
	})
}

func newRegisterLimiter(cache store.Cache) fiber.Handler {
	return handler.RateLimiter(handler.RateLimiterConfig{
		Max: 5, Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string { return handler.GetRealIPForRateLimit(c, "register:") },
		Cache:        cache,
	})
}

func authConfigHandler(settings *store.SettingStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		payload := fiber.Map{
			"register_enabled":           settings.GetBool(store.SettingRegisterEnabled, true),
			"turnstile_site_key":         settings.Get(store.SettingTurnstileSiteKey, ""),
			"email_verification_enabled": settings.GetBool(store.SettingEmailVerificationEnabled, false),
			"site_title":                 settings.Get(store.SettingSiteTitle, ""),
		}
		return c.JSON(payload)
	}
}

func registerEnabledMiddleware(settings *store.SettingStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !settings.GetBool(store.SettingRegisterEnabled, true) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "registration is disabled"})
		}
		return c.Next()
	}
}

func managedRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	routes := make([]managedRouteSpec, 0)
	routes = append(routes, authRouteSpecs(rt)...)
	routes = append(routes, oauthRouteSpecs(rt)...)
	routes = append(routes, userRouteSpecs(rt)...)
	routes = append(routes, pointsRouteSpecs(rt)...)
	routes = append(routes, adminRouteSpecs(rt)...)
	routes = append(routes, githubCompatRouteSpecs(rt)...)
	slices.SortFunc(routes, func(left, right managedRouteSpec) int {
		if left.Doc.Path == right.Doc.Path {
			return strings.Compare(left.Doc.Method, right.Doc.Method)
		}
		return strings.Compare(left.Doc.Path, right.Doc.Path)
	})
	return routes
}

func managedOpenAPIRoutes() []openAPIRoute {
	specs := managedRouteSpecs(managedRouteRuntime{})
	routes := make([]openAPIRoute, 0, len(specs))
	for _, spec := range specs {
		routes = append(routes, spec.Doc)
	}
	return routes
}

func registerManagedRoutes(app *fiber.App, rt managedRouteRuntime) {
	for _, spec := range managedRouteSpecs(rt) {
		handlers := spec.Handlers(rt)
		if len(handlers) == 0 {
			continue
		}
		app.Add([]string{spec.Doc.Method}, spec.Doc.Path, handlers[0], toAnySlice(handlers[1:])...)
	}
}

func toAnySlice(handlers []fiber.Handler) []any {
	values := make([]any, 0, len(handlers))
	for _, handlerFunc := range handlers {
		values = append(values, handlerFunc)
	}
	return values
}

func turnstileMiddleware(settings *store.SettingStore) fiber.Handler {
	client := &http.Client{Timeout: 3 * time.Second}

	return func(c fiber.Ctx) error {
		secretKey := settings.Get(store.SettingTurnstileSecretKey, "")
		if secretKey == "" {
			return c.Next()
		}

		var body struct {
			TurnstileToken string `json:"turnstile_token"`
		}
		if err := json.Unmarshal(c.Body(), &body); err != nil || body.TurnstileToken == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "turnstile verification required"})
		}

		form := url.Values{}
		form.Set("secret", secretKey)
		form.Set("response", body.TurnstileToken)
		form.Set("remoteip", handler.GetRealIP(c))
		request, err := http.NewRequestWithContext(c.Context(), http.MethodPost,
			"https://challenges.cloudflare.com/turnstile/v0/siteverify",
			strings.NewReader(form.Encode()))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "turnstile verification failed"})
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		response, err := client.Do(request)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "turnstile verification failed"})
		}
		defer response.Body.Close()

		var result struct {
			Success bool `json:"success"`
		}
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil || !result.Success {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "turnstile verification failed"})
		}
		return c.Next()
	}
}
