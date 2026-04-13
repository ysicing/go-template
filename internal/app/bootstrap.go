package app

import (
	"context"
	"crypto/sha256"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/internal/service"
	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

func validateSecurityConfig(cfg *Config, log *zerolog.Logger) {
	if cfg.IsDefaultSecret() && !cfg.Security.AllowInsecure {
		log.Fatal().Msg("default JWT secret is not allowed in secure mode, set jwt.secret or security.allow_insecure: true")
	}
	if cfg.IsDefaultSecret() && cfg.Security.AllowInsecure {
		log.Warn().Msg("using default JWT secret")
	}
	if cfg.Security.EncryptionKey == "" {
		log.Warn().Msg("security.encryption_key is not set — TOTP secrets and social provider credentials will be stored in plaintext")
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
		DB:                     db,
		Cache:                  cache,
		UserStore:              store.NewUserStore(db),
		PasswordHistory:        store.NewPasswordHistoryStore(db),
		ClientStore:            store.NewOAuthClientStore(db),
		OAuthConsentGrantStore: store.NewOAuthConsentGrantStore(db),
		SocialStore:            store.NewSocialProviderStore(db, cfg.Security.EncryptionKey),
		SocialAccountStore:     store.NewSocialAccountStore(db),
		SettingStore:           store.NewSettingStore(db, cache),
		RefreshTokenStore:      store.NewAPIRefreshTokenStore(db),
		AuditLogStore:          store.NewAuditLogStore(db),
		MFAStore:               store.NewMFAStore(db, cfg.Security.EncryptionKey),
		WebAuthnStore:          store.NewWebAuthnStore(db),
		PointStore:             store.NewPointStore(db),
	}
	deps.CheckInStore = store.NewCheckInStore(db, deps.PointStore)
	deps.Services = Services{
		ClientCredentials: service.NewClientCredentialsService(service.ClientCredentialsServiceDeps{
			DB:      db,
			Clients: deps.ClientStore,
			Audit:   deps.AuditLogStore,
		}),
		Sessions: service.NewSessionService(deps.RefreshTokenStore, service.TokenConfig{
			Secret:        cfg.JWT.Secret,
			Issuer:        cfg.JWT.Issuer,
			AccessTTL:     cfg.JWT.AccessTokenTTL,
			RefreshTTL:    cfg.JWT.RefreshTokenTTL,
			RememberMeTTL: cfg.JWT.RememberMeTTL,
		}),
	}
	deps.Services.Auth = service.NewAuthService(service.AuthServiceDeps{
		Users: deps.UserStore,
		Cache: cache,
	})

	oidcStorage, err := store.NewOIDCStorage(ctx, db, cache, deps.UserStore, deps.ClientStore, "/login", cfg.Security.EncryptionKey, 0, 0, 0)
	if err != nil {
		log.Fatal().Err(err).Msg("create oidc storage")
	}
	deps.OIDCStorage = oidcStorage
	return deps
}

func initOIDCProvider(cfg *Config, deps *Deps, log *zerolog.Logger) *op.Provider {
	oidcSecretSource := cfg.Security.OIDCSecret
	if oidcSecretSource == "" {
		oidcSecretSource = cfg.JWT.Secret + ":oidc"
		log.Warn().Msg("security.oidc_secret not set, deriving from JWT secret with salt")
	}
	cryptoKey := sha256.Sum256([]byte(oidcSecretSource))
	opOpts := []op.Option{}
	if cfg.Security.AllowInsecure {
		opOpts = append(opOpts, op.WithAllowInsecure())
	}

	provider, err := op.NewProvider(
		&op.Config{CryptoKey: cryptoKey},
		deps.OIDCStorage,
		op.IssuerFromForwardedOrHost(""),
		opOpts...,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("create oidc provider")
	}
	return provider
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
