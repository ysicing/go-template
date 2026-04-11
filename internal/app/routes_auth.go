package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/store"
)

func registerAuthModule(
	api fiber.Router,
	d *Deps,
	h *builtHandlers,
	jwtMW fiber.Handler,
	tokenVersionMW fiber.Handler,
	optionalJWT fiber.Handler,
) {
	authLimiter := handler.RateLimiter(handler.RateLimiterConfig{
		Max: 20, Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string { return handler.GetRealIPForRateLimit(c, "auth:") },
		Cache:        d.Cache,
	})
	ts := turnstileMiddleware(d.SettingStore)
	authGroup := api.Group("/auth", authLimiter)

	authGroup.Get("/config", func(c fiber.Ctx) error {
		payload := fiber.Map{
			"register_enabled":           d.SettingStore.GetBool(store.SettingRegisterEnabled, true),
			"turnstile_site_key":         d.SettingStore.Get(store.SettingTurnstileSiteKey, ""),
			"email_verification_enabled": d.SettingStore.GetBool(store.SettingEmailVerificationEnabled, false),
			"site_title":                 d.SettingStore.Get(store.SettingSiteTitle, ""),
		}
		return c.JSON(payload)
	})
	authGroup.Post("/register", newRegisterLimiter(d.Cache), func(c fiber.Ctx) error {
		if !d.SettingStore.GetBool(store.SettingRegisterEnabled, true) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "registration is disabled"})
		}
		return c.Next()
	}, ts, h.auth.Register)
	authGroup.Post("/login", ts, h.auth.Login)
	authGroup.Post("/refresh", h.auth.Refresh)
	authGroup.Post("/oidc-login", h.oidcLogin.LoginSubmit)
	authGroup.Get("/oidc/consent", h.oidcLogin.ConsentContext)
	authGroup.Post("/oidc/consent/approve", h.oidcLogin.ConsentApprove)
	authGroup.Post("/oidc/consent/deny", h.oidcLogin.ConsentDeny)
	authGroup.Post("/verify-email", h.email.VerifyEmail)
	authGroup.Post("/mfa/verify", h.mfa.Verify)
	authGroup.Post("/webauthn/begin", h.webauthn.LoginBegin)
	authGroup.Post("/webauthn/finish", h.webauthn.LoginFinish)
	authGroup.Post("/mfa/webauthn/begin", h.webauthn.AuthBegin)
	authGroup.Post("/mfa/webauthn/finish", h.webauthn.AuthFinish)
	authGroup.Get("/github", optionalJWT, h.oauth.GitHubLogin)
	authGroup.Get("/github/callback", h.oauth.GitHubCallback)
	authGroup.Get("/google", optionalJWT, h.oauth.GoogleLogin)
	authGroup.Get("/google/callback", h.oauth.GoogleCallback)
	authGroup.Post("/social/exchange", h.oauth.ExchangeCode)
	authGroup.Post("/social/confirm-link", h.oauth.ConfirmSocialLink)
	authGroup.Post("/logout", optionalJWT, h.auth.Logout)
	authGroup.Post("/resend-verification", jwtMW, tokenVersionMW, h.email.ResendVerification)
}

func registerGitHubCompatRoutes(app *fiber.App, h *builtHandlers, ghLimiter fiber.Handler) {
	app.Get("/login/oauth/authorize", ghLimiter, h.ghCompat.Authorize)
	app.Post("/login/oauth/access_token", ghLimiter, h.ghCompat.AccessToken)
	app.Get("/api/v3/user", ghLimiter, h.ghCompat.User)
	app.Get("/api/v3/user/emails", ghLimiter, h.ghCompat.UserEmails)
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
		req, err := http.NewRequestWithContext(c.Context(), http.MethodPost,
			"https://challenges.cloudflare.com/turnstile/v0/siteverify",
			strings.NewReader(form.Encode()))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "turnstile verification failed"})
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "turnstile verification failed"})
		}
		defer resp.Body.Close()

		var result struct {
			Success bool `json:"success"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || !result.Success {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "turnstile verification failed"})
		}

		return c.Next()
	}
}
