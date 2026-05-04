package app

import (
	adminhandler "github.com/ysicing/go-template/handler/admin"
	authhandler "github.com/ysicing/go-template/handler/auth"
	emailhandler "github.com/ysicing/go-template/handler/email"
	mfahandler "github.com/ysicing/go-template/handler/mfa"
	oauthhandler "github.com/ysicing/go-template/handler/oauth"
	pointshandler "github.com/ysicing/go-template/handler/points"
	socialaccounthandler "github.com/ysicing/go-template/handler/socialaccount"
	userhandler "github.com/ysicing/go-template/handler/user"
	webauthnhandler "github.com/ysicing/go-template/handler/webauthn"
	"github.com/ysicing/go-template/internal/queue"
	authservice "github.com/ysicing/go-template/internal/service/auth"
	sessionservice "github.com/ysicing/go-template/internal/service/session"
	"github.com/ysicing/go-template/store"
	pointstore "github.com/ysicing/go-template/store/points"
	webauthnstore "github.com/ysicing/go-template/store/webauthn"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// Services groups application-level services shared across handlers.
type Services struct {
	Sessions *sessionservice.SessionService
	Auth     *authservice.AuthService
}

// Deps aggregates all dependencies needed by the route setup.
type Deps struct {
	Config             *Config
	DB                 *gorm.DB
	UserStore          *store.UserStore
	PasswordHistory    *store.PasswordHistoryStore
	SocialStore        *store.SocialProviderStore
	SocialAccountStore *store.SocialAccountStore
	SettingStore       *store.SettingStore
	RefreshTokenStore  *store.APIRefreshTokenStore
	AuditLogStore      *store.AuditLogStore
	MFAStore           *store.MFAStore
	WebAuthnStore      *webauthnstore.WebAuthnStore
	PointStore         *pointstore.PointStore
	CheckInStore       *pointstore.CheckInStore
	Cache              store.Cache
	TaskQueue          queue.Enqueuer
	TaskServer         *queue.Server
	Services           Services
}

// builtHandlers holds all handler instances created during route setup.
type builtHandlers struct {
	auth        *authhandler.AuthHandler
	email       *emailhandler.EmailHandler
	user        *userhandler.UserHandler
	mfa         *mfahandler.MFAHandler
	webauthn    *webauthnhandler.WebAuthnHandler
	oauth       *oauthhandler.OAuthHandler
	socialAcct  *socialaccounthandler.SocialAccountHandler
	admin       *adminhandler.AdminHandler
	adminProv   *adminhandler.AdminProviderHandler
	adminSett   *adminhandler.AdminSettingHandler
	adminPoints *adminhandler.AdminPointsHandler
	points      *pointshandler.PointsHandler
}

func buildAllHandlers(d *Deps, tokenCfg sessionservice.TokenConfig) *builtHandlers {
	h := &builtHandlers{}
	buildIdentityHandlers(h, d, tokenCfg)
	buildOAuthHandlers(h, d, tokenCfg)
	buildAdminHandlers(h, d)
	return h
}

func buildIdentityHandlers(h *builtHandlers, d *Deps, tokenCfg sessionservice.TokenConfig) {
	h.email = emailhandler.NewEmailHandler(d.UserStore, d.SettingStore, d.AuditLogStore, d.PointStore, d.Cache)
	if d.TaskQueue != nil {
		h.email.SetQueue(d.TaskQueue)
	}
	h.auth = authhandler.NewAuthHandler(authhandler.AuthDeps{
		Users:         d.UserStore,
		WebAuthnCreds: d.WebAuthnStore,
		RefreshTokens: d.RefreshTokenStore,
		Sessions:      d.Services.Sessions,
		AuthService:   d.Services.Auth,
		MFA:           d.MFAStore,
		Audit:         d.AuditLogStore,
		Cache:         d.Cache,
		Settings:      d.SettingStore,
		TokenConfig:   tokenCfg,
	})
	h.auth.SetEmailHandler(h.email)

	h.user = userhandler.NewUserHandler(userhandler.UserDeps{
		Users:           d.UserStore,
		PasswordHistory: d.PasswordHistory,
		RefreshTokens:   d.RefreshTokenStore,
		Audit:           d.AuditLogStore,
		Settings:        d.SettingStore,
		EmailHandler:    h.email,
		Cache:           d.Cache,
	})

	h.mfa = mfahandler.NewMFAHandler(mfahandler.MFADeps{
		Users:         d.UserStore,
		MFA:           d.MFAStore,
		Audit:         d.AuditLogStore,
		RefreshTokens: d.RefreshTokenStore,
		Sessions:      d.Services.Sessions,
		Cache:         d.Cache,
		TokenConfig:   tokenCfg,
	})

	h.webauthn = webauthnhandler.NewWebAuthnHandler(webauthnhandler.WebAuthnDeps{
		Settings:      d.SettingStore,
		Users:         d.UserStore,
		Creds:         d.WebAuthnStore,
		MFA:           d.MFAStore,
		Audit:         d.AuditLogStore,
		RefreshTokens: d.RefreshTokenStore,
		Sessions:      d.Services.Sessions,
		Cache:         d.Cache,
		TokenConfig:   tokenCfg,
	})
}

func buildOAuthHandlers(h *builtHandlers, d *Deps, tokenCfg sessionservice.TokenConfig) {
	h.oauth = oauthhandler.NewOAuthHandler(oauthhandler.OAuthDeps{
		Users:          d.UserStore,
		Providers:      d.SocialStore,
		SocialAccounts: d.SocialAccountStore,
		Audit:          d.AuditLogStore,
		RefreshTokens:  d.RefreshTokenStore,
		Sessions:       d.Services.Sessions,
		MFA:            d.MFAStore,
		Cache:          d.Cache,
		Settings:       d.SettingStore,
		WebAuthnCreds:  d.WebAuthnStore,
		TokenConfig:    tokenCfg,
	})

	h.socialAcct = socialaccounthandler.NewSocialAccountHandler(d.SocialAccountStore, d.UserStore, d.AuditLogStore, nil)
	h.points = pointshandler.NewPointsHandler(d.PointStore, d.CheckInStore, d.AuditLogStore)
}

func buildAdminHandlers(h *builtHandlers, d *Deps) {
	h.admin = adminhandler.NewAdminHandler(adminhandler.AdminDeps{
		Users:          d.UserStore,
		Audit:          d.AuditLogStore,
		RefreshTokens:  d.RefreshTokenStore,
		MFA:            d.MFAStore,
		WebAuthnCreds:  d.WebAuthnStore,
		SocialAccounts: d.SocialAccountStore,
		PasswordHist:   d.PasswordHistory,
		Cache:          d.Cache,
	})
	h.adminProv = adminhandler.NewAdminProviderHandler(d.SocialStore, d.AuditLogStore)
	h.adminSett = adminhandler.NewAdminSettingHandler(d.SettingStore, d.AuditLogStore, h.email)
	h.adminPoints = adminhandler.NewAdminPointsHandler(d.PointStore, d.AuditLogStore)
}

// SetupRoutes registers all API routes on the Fiber app.
func SetupRoutes(app *fiber.App, d *Deps) {
	cfg := d.Config
	tokenCfg := sessionservice.TokenConfig{
		Secret:        cfg.JWT.Secret,
		Issuer:        cfg.JWT.Issuer,
		AccessTTL:     cfg.JWT.AccessTokenTTL,
		RefreshTTL:    cfg.JWT.RefreshTokenTTL,
		RememberMeTTL: cfg.JWT.RememberMeTTL,
	}

	h := buildAllHandlers(d, tokenCfg)
	registerTaskHandlers(d, h)

	registerManagedRoutes(app, buildManagedRouteRuntime(d, h))
}

func registerTaskHandlers(d *Deps, h *builtHandlers) {
	if d.TaskServer == nil || h.email == nil {
		return
	}
	d.TaskServer.HandleFunc(emailhandler.TypeVerificationEmailTask, h.email.HandleVerificationEmailTask)
}
