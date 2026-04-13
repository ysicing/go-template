package app

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/store"
)

// Services groups application-level services shared across handlers.
type Services struct {
	ClientCredentials *service.ClientCredentialsService
	Sessions          *service.SessionService
	Auth              *service.AuthService
}

// Deps aggregates all dependencies needed by the route setup.
type Deps struct {
	Config                 *Config
	DB                     *gorm.DB
	UserStore              *store.UserStore
	PasswordHistory        *store.PasswordHistoryStore
	ClientStore            *store.OAuthClientStore
	OAuthConsentGrantStore *store.OAuthConsentGrantStore
	OIDCStorage            *store.OIDCStorage
	SocialStore            *store.SocialProviderStore
	SocialAccountStore     *store.SocialAccountStore
	SettingStore           *store.SettingStore
	RefreshTokenStore      *store.APIRefreshTokenStore
	AuditLogStore          *store.AuditLogStore
	MFAStore               *store.MFAStore
	WebAuthnStore          *store.WebAuthnStore
	PointStore             *store.PointStore
	CheckInStore           *store.CheckInStore
	Cache                  store.Cache
	OIDCHandler            http.Handler
	Services               Services
}

// builtHandlers holds all handler instances created during route setup.
type builtHandlers struct {
	auth              *handler.AuthHandler
	email             *handler.EmailHandler
	user              *handler.UserHandler
	mfa               *handler.MFAHandler
	webauthn          *handler.WebAuthnHandler
	oauth             *handler.OAuthHandler
	oidcLogin         *handler.OIDCLoginHandler
	socialAcct        *handler.SocialAccountHandler
	oauthClient       *handler.OAuthClientHandler
	admin             *handler.AdminHandler
	adminProv         *handler.AdminProviderHandler
	adminSett         *handler.AdminSettingHandler
	adminPoints       *handler.AdminPointsHandler
	points            *handler.PointsHandler
	ghCompat          *handler.GitHubCompatHandler
	clientCredentials *handler.ClientCredentialsHandler
}

func buildAllHandlers(d *Deps, tokenCfg handler.TokenConfig) *builtHandlers {
	h := &builtHandlers{}

	h.email = handler.NewEmailHandler(d.UserStore, d.SettingStore, d.AuditLogStore, d.PointStore, d.Cache)
	h.auth = handler.NewAuthHandler(handler.AuthDeps{
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

	h.user = handler.NewUserHandler(handler.UserDeps{
		Users:           d.UserStore,
		PasswordHistory: d.PasswordHistory,
		RefreshTokens:   d.RefreshTokenStore,
		Audit:           d.AuditLogStore,
		ConsentGrants:   d.OAuthConsentGrantStore,
		Clients:         d.ClientStore,
		Settings:        d.SettingStore,
		EmailHandler:    h.email,
		Cache:           d.Cache,
	})

	h.mfa = handler.NewMFAHandler(handler.MFADeps{
		Users:         d.UserStore,
		MFA:           d.MFAStore,
		Audit:         d.AuditLogStore,
		RefreshTokens: d.RefreshTokenStore,
		Cache:         d.Cache,
		OIDC:          d.OIDCStorage,
		Clients:       d.ClientStore,
		ConsentGrants: d.OAuthConsentGrantStore,
		TokenConfig:   tokenCfg,
	})

	h.webauthn = handler.NewWebAuthnHandler(handler.WebAuthnDeps{
		Settings:      d.SettingStore,
		Users:         d.UserStore,
		Creds:         d.WebAuthnStore,
		MFA:           d.MFAStore,
		Audit:         d.AuditLogStore,
		RefreshTokens: d.RefreshTokenStore,
		Cache:         d.Cache,
		TokenConfig:   tokenCfg,
	})

	h.oauth = handler.NewOAuthHandler(handler.OAuthDeps{
		DB:             d.DB,
		Users:          d.UserStore,
		Providers:      d.SocialStore,
		SocialAccounts: d.SocialAccountStore,
		Audit:          d.AuditLogStore,
		RefreshTokens:  d.RefreshTokenStore,
		MFA:            d.MFAStore,
		Cache:          d.Cache,
		Settings:       d.SettingStore,
		TokenConfig:    tokenCfg,
	})

	h.oidcLogin = handler.NewOIDCLoginHandler(d.OIDCStorage, d.ClientStore, d.OAuthConsentGrantStore, d.UserStore, d.MFAStore, d.AuditLogStore, d.Cache)
	h.socialAcct = handler.NewSocialAccountHandler(d.SocialAccountStore, d.UserStore, d.AuditLogStore, nil)
	h.oauthClient = handler.NewOAuthClientHandler(d.ClientStore, d.AuditLogStore)
	h.points = handler.NewPointsHandler(d.PointStore, d.CheckInStore, d.AuditLogStore)

	h.admin = handler.NewAdminHandler(handler.AdminDeps{
		Users:          d.UserStore,
		Clients:        d.ClientStore,
		Audit:          d.AuditLogStore,
		RefreshTokens:  d.RefreshTokenStore,
		MFA:            d.MFAStore,
		WebAuthnCreds:  d.WebAuthnStore,
		SocialAccounts: d.SocialAccountStore,
		PasswordHist:   d.PasswordHistory,
		Cache:          d.Cache,
		DB:             d.DB,
	})
	h.adminProv = handler.NewAdminProviderHandler(d.SocialStore, d.AuditLogStore)
	h.adminSett = handler.NewAdminSettingHandler(d.SettingStore, d.AuditLogStore, h.email)
	h.adminPoints = handler.NewAdminPointsHandler(d.PointStore, d.AuditLogStore)
	h.ghCompat = handler.NewGitHubCompatHandler(d.OIDCHandler, d.OIDCStorage)
	h.clientCredentials = handler.NewClientCredentialsHandler(d.Services.ClientCredentials, d.OIDCHandler)

	return h
}

// SetupRoutes registers all API routes on the Fiber app.
func SetupRoutes(app *fiber.App, d *Deps) {
	cfg := d.Config
	tokenCfg := handler.TokenConfig{
		Secret:        cfg.JWT.Secret,
		Issuer:        cfg.JWT.Issuer,
		AccessTTL:     cfg.JWT.AccessTokenTTL,
		RefreshTTL:    cfg.JWT.RefreshTokenTTL,
		RememberMeTTL: cfg.JWT.RememberMeTTL,
	}

	h := buildAllHandlers(d, tokenCfg)

	registerManagedRoutes(app, buildManagedRouteRuntime(d, h))
}
