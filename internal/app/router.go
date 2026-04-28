package app

import (
	"net/http"

	"github.com/ysicing/go-template/handler"
	clientcredentialshandler "github.com/ysicing/go-template/handler/clientcredentials"
	emailhandler "github.com/ysicing/go-template/handler/email"
	githubcompathandler "github.com/ysicing/go-template/handler/githubcompat"
	mfahandler "github.com/ysicing/go-template/handler/mfa"
	oauthhandler "github.com/ysicing/go-template/handler/oauth"
	oauthclienthandler "github.com/ysicing/go-template/handler/oauthclient"
	pointshandler "github.com/ysicing/go-template/handler/points"
	socialaccounthandler "github.com/ysicing/go-template/handler/socialaccount"
	webauthnhandler "github.com/ysicing/go-template/handler/webauthn"
	authservice "github.com/ysicing/go-template/internal/service/auth"
	clientcredentialsservice "github.com/ysicing/go-template/internal/service/clientcredentials"
	sessionservice "github.com/ysicing/go-template/internal/service/session"
	"github.com/ysicing/go-template/store"
	oidcstore "github.com/ysicing/go-template/store/oidc"
	pointstore "github.com/ysicing/go-template/store/points"
	webauthnstore "github.com/ysicing/go-template/store/webauthn"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// Services groups application-level services shared across handlers.
type Services struct {
	ClientCredentials *clientcredentialsservice.ClientCredentialsService
	Sessions          *sessionservice.SessionService
	Auth              *authservice.AuthService
}

// Deps aggregates all dependencies needed by the route setup.
type Deps struct {
	Config                 *Config
	DB                     *gorm.DB
	UserStore              *store.UserStore
	PasswordHistory        *store.PasswordHistoryStore
	ClientStore            *store.OAuthClientStore
	OAuthConsentGrantStore *store.OAuthConsentGrantStore
	OIDCStorage            *oidcstore.OIDCStorage
	SocialStore            *store.SocialProviderStore
	SocialAccountStore     *store.SocialAccountStore
	SettingStore           *store.SettingStore
	RefreshTokenStore      *store.APIRefreshTokenStore
	AuditLogStore          *store.AuditLogStore
	MFAStore               *store.MFAStore
	WebAuthnStore          *webauthnstore.WebAuthnStore
	PointStore             *pointstore.PointStore
	CheckInStore           *pointstore.CheckInStore
	Cache                  store.Cache
	OIDCHandler            http.Handler
	Services               Services
}

// builtHandlers holds all handler instances created during route setup.
type builtHandlers struct {
	auth              *handler.AuthHandler
	email             *emailhandler.EmailHandler
	user              *handler.UserHandler
	mfa               *mfahandler.MFAHandler
	webauthn          *webauthnhandler.WebAuthnHandler
	oauth             *oauthhandler.OAuthHandler
	oidcLogin         *handler.OIDCLoginHandler
	socialAcct        *socialaccounthandler.SocialAccountHandler
	oauthClient       *oauthclienthandler.OAuthClientHandler
	admin             *handler.AdminHandler
	adminProv         *handler.AdminProviderHandler
	adminSett         *handler.AdminSettingHandler
	adminPoints       *handler.AdminPointsHandler
	points            *pointshandler.PointsHandler
	ghCompat          *githubcompathandler.GitHubCompatHandler
	clientCredentials *clientcredentialshandler.ClientCredentialsHandler
}

func buildAllHandlers(d *Deps, tokenCfg handler.TokenConfig) *builtHandlers {
	h := &builtHandlers{}
	buildIdentityHandlers(h, d, tokenCfg)
	buildOAuthHandlers(h, d, tokenCfg)
	buildAdminHandlers(h, d)
	return h
}

func buildIdentityHandlers(h *builtHandlers, d *Deps, tokenCfg handler.TokenConfig) {
	h.email = emailhandler.NewEmailHandler(d.UserStore, d.SettingStore, d.AuditLogStore, d.PointStore, d.Cache)
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

	h.mfa = mfahandler.NewMFAHandler(mfahandler.MFADeps{
		Users:         d.UserStore,
		MFA:           d.MFAStore,
		Audit:         d.AuditLogStore,
		RefreshTokens: d.RefreshTokenStore,
		Sessions:      d.Services.Sessions,
		Cache:         d.Cache,
		OIDC:          d.OIDCStorage,
		Clients:       d.ClientStore,
		ConsentGrants: d.OAuthConsentGrantStore,
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

func buildOAuthHandlers(h *builtHandlers, d *Deps, tokenCfg handler.TokenConfig) {
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

	h.oidcLogin = handler.NewOIDCLoginHandler(d.OIDCStorage, d.ClientStore, d.OAuthConsentGrantStore, d.UserStore, d.MFAStore, d.AuditLogStore, d.Cache)
	h.socialAcct = socialaccounthandler.NewSocialAccountHandler(d.SocialAccountStore, d.UserStore, d.AuditLogStore, nil)
	h.oauthClient = oauthclienthandler.NewOAuthClientHandler(d.ClientStore, d.AuditLogStore)
	h.points = pointshandler.NewPointsHandler(d.PointStore, d.CheckInStore, d.AuditLogStore)
	h.ghCompat = githubcompathandler.NewGitHubCompatHandler(d.OIDCHandler, d.OIDCStorage)
	h.clientCredentials = clientcredentialshandler.NewClientCredentialsHandler(d.Services.ClientCredentials, d.OIDCHandler)
}

func buildAdminHandlers(h *builtHandlers, d *Deps) {
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
	})
	h.adminProv = handler.NewAdminProviderHandler(d.SocialStore, d.AuditLogStore)
	h.adminSett = handler.NewAdminSettingHandler(d.SettingStore, d.AuditLogStore, h.email)
	h.adminPoints = handler.NewAdminPointsHandler(d.PointStore, d.AuditLogStore)
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
