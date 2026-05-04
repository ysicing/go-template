package app

import (
	"context"

	authservice "github.com/ysicing/go-template/internal/service/auth"
	clientcredentialsservice "github.com/ysicing/go-template/internal/service/clientcredentials"
	sessionservice "github.com/ysicing/go-template/internal/service/session"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
	pointstore "github.com/ysicing/go-template/store/points"
	webauthnstore "github.com/ysicing/go-template/store/webauthn"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func validateSecurityConfig(cfg *Config, log *zerolog.Logger) {
	if cfg.IsDemoMode() {
		log.Warn().Msg("running in DEMO mode — default secrets and relaxed policies are allowed; do NOT use in production")
		cfg.Security.AllowInsecure = true
		return
	}
	if cfg.IsDefaultSecret() {
		log.Fatal().Msg("default JWT secret is not allowed outside demo mode, set jwt.secret or security.mode: demo")
	}
	if cfg.Security.EncryptionKey == "" {
		log.Fatal().Msg("security.encryption_key is required outside demo mode — TOTP secrets and social provider credentials need encryption")
	}
}

func initDBAndCache(ctx context.Context, cfg *Config, log *zerolog.Logger) (*gorm.DB, store.Cache) {
	db, err := store.InitDB(cfg.Database.Driver, cfg.Database.DSN, cfg.Log.Level)
	if err != nil {
		log.Fatal().Err(err).Msg("init db")
	}
	if err := model.Migrate(db); err != nil {
		log.Fatal().Err(err).Msg("migrate")
	}
	if err := store.VerifySQLiteWritable(db); err != nil {
		log.Fatal().Err(err).Msg("sqlite database is not writable; check database.dsn path permissions")
	}

	cache := initCache(ctx, cfg, log)
	return db, cache
}

func initCache(ctx context.Context, cfg *Config, log *zerolog.Logger) store.Cache {
	if cfg.Redis.Addr == "" {
		log.Info().Msg("cache backend: memory (single-replica/development)")
		return store.NewMemoryCache()
	}

	redisCache := store.NewCache(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err := redisCache.Ping(ctx); err != nil {
		log.Error().Err(err).Str("addr", cfg.Redis.Addr).Int("db", cfg.Redis.DB).Msg("cache ping failed, fallback to memory")
		_ = redisCache.Close()
		log.Warn().Msg("cache backend: memory (fallback, not suitable for multi-replica sharing)")
		return store.NewMemoryCache()
	}

	log.Info().Str("addr", cfg.Redis.Addr).Int("db", cfg.Redis.DB).Msg("cache backend: redis (multi-replica ready)")
	return redisCache
}

func initDeps(ctx context.Context, db *gorm.DB, cache store.Cache, cfg *Config, log *zerolog.Logger) *Deps {
	deps := &Deps{
		DB:                 db,
		Cache:              cache,
		UserStore:          store.NewUserStore(db),
		PasswordHistory:    store.NewPasswordHistoryStore(db),
		ClientStore:        store.NewOAuthClientStore(db),
		SocialStore:        store.NewSocialProviderStore(db, cfg.Security.EncryptionKey),
		SocialAccountStore: store.NewSocialAccountStore(db),
		SettingStore:       store.NewSettingStore(db, cache, cfg.Security.EncryptionKey),
		RefreshTokenStore:  store.NewAPIRefreshTokenStore(db, cache),
		AuditLogStore:      store.NewAuditLogStore(db),
		MFAStore:           store.NewMFAStore(db, cfg.Security.EncryptionKey),
		WebAuthnStore:      webauthnstore.NewWebAuthnStore(db),
		PointStore:         pointstore.NewPointStore(db),
	}
	deps.CheckInStore = pointstore.NewCheckInStore(db, deps.PointStore)
	deps.Services = Services{
		ClientCredentials: clientcredentialsservice.NewClientCredentialsService(clientcredentialsservice.ClientCredentialsServiceDeps{
			Clients: deps.ClientStore,
			Audit:   deps.AuditLogStore,
		}),
		Sessions: sessionservice.NewSessionService(deps.RefreshTokenStore, sessionservice.TokenConfig{
			Secret:        cfg.JWT.Secret,
			Issuer:        cfg.JWT.Issuer,
			AccessTTL:     cfg.JWT.AccessTokenTTL,
			RefreshTTL:    cfg.JWT.RefreshTokenTTL,
			RememberMeTTL: cfg.JWT.RememberMeTTL,
		}),
	}
	deps.Services.Auth = authservice.NewAuthService(authservice.AuthServiceDeps{
		Users: deps.UserStore,
		Cache: cache,
	})
	return deps
}

func newFiberApp(cfg *Config, log *zerolog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:     "go-template",
		TrustProxy:  len(cfg.Server.TrustedProxies) > 0,
		ProxyHeader: fiber.HeaderXForwardedFor,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: cfg.Server.TrustedProxies, // Only trust explicitly configured proxies
		},
	})
	if len(cfg.Server.TrustedProxies) == 0 {
		log.Warn().Msg("No trusted proxies configured - X-Forwarded-For headers will be ignored. Set server.trusted_proxies in config or TRUSTED_PROXIES env var.")
	}
	return app
}

func registerSystemRoutes(app *fiber.App, buildInfo BuildInfo) {
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/api/version", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"version":    buildInfo.Version,
			"git_commit": buildInfo.GitCommit,
			"build_date": buildInfo.BuildDate,
		})
	})
}
