package app

import (
	"context"
	"io/fs"
	"time"

	"github.com/rs/zerolog"

	"github.com/ysicing/go-template/handler"
	"github.com/ysicing/go-template/pkg/metrics"
)

// Run wires dependencies and starts the service lifecycle.
func Run(ctx context.Context, cfg *Config, webDistFS fs.FS, buildInfo BuildInfo, log *zerolog.Logger) {
	validateSecurityConfig(cfg, log)
	db, cache := initDBAndCache(ctx, cfg, log)
	deps := initDeps(ctx, db, cache, cfg, log)

	seedAdmin(log, deps.UserStore, cfg.Admin)
	go cleanupAPIRefreshTokens(ctx, log, deps.RefreshTokenStore)
	metrics.StartSystemMetricsCollector(ctx, 15*time.Second)

	provider := initOIDCProvider(cfg, deps, log)

	handler.SetTrustedProxies(cfg.Server.TrustedProxies)
	fiberApp := newFiberApp(cfg, log)
	setupMiddlewareChain(fiberApp, cfg, deps.SettingStore, log)

	deps.Config = cfg
	deps.OIDCHandler = provider
	registerSystemRoutes(fiberApp, buildInfo)
	registerDocsRoutes(fiberApp, deps, buildInfo)
	SetupRoutes(fiberApp, deps)
	mountOIDCHandler(fiberApp, provider)
	mountSPA(fiberApp, webDistFS)

	runServer(fiberApp, cache, cfg.Server.Addr, buildInfo, log)
}
