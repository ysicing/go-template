package app

import (
	"context"
	"io/fs"
	"time"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/pkg/metrics"
	"github.com/ysicing/go-template/store"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// Run wires dependencies and starts the service lifecycle.
func Run(ctx context.Context, cfg *Config, webDistFS fs.FS, buildInfo BuildInfo, log *zerolog.Logger) {
	validateSecurityConfig(cfg, log)
	db, cache := initDBAndCache(ctx, cfg, log)
	sessionStorage := store.NewSessionStorageResource(cache)
	deps := initDeps(ctx, db, cache, cfg, log)

	seedAdmin(log, deps.UserStore, cfg.Admin)
	go cleanupAPIRefreshTokens(ctx, log, deps.RefreshTokenStore)
	metrics.StartSystemMetricsCollector(ctx, 15*time.Second)

	provider := initOIDCProvider(cfg, deps, log)

	handler.SetTrustedProxies(cfg.Server.TrustedProxies)
	fiberApp := newFiberApp(cfg, log)
	setupMiddlewareChain(fiberApp, deps.SettingStore, sessionStorage.Storage, log)

	deps.Config = cfg
	deps.OIDCHandler = provider
	registerSystemRoutes(fiberApp, buildInfo)
	registerDocsRoutes(fiberApp, deps, buildInfo)
	SetupRoutes(fiberApp, deps)
	mountOIDCHandler(fiberApp, provider)
	mountSPA(fiberApp, webDistFS)

	runServer(fiberApp, runtimeResources{
		db:             db,
		cache:          cache,
		sessionStorage: sessionStorage,
	}, cfg.Server.Addr, buildInfo, log)
}

type runtimeResources struct {
	db             *gorm.DB
	cache          store.Cache
	sessionStorage store.SessionStorageResource
}
