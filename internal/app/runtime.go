package app

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/pkg/logger"
	"github.com/ysicing/go-template/store"
)

func seedAdmin(log *zerolog.Logger, userStore *store.UserStore, cfg AdminSeedConfig) {
	if cfg.Username == "" {
		return
	}
	if cfg.Password == "" {
		log.Warn().Str("username", cfg.Username).Msg("skip admin seed because admin.password is empty")
		return
	}
	_, err := userStore.GetByUsername(context.Background(), cfg.Username)
	if err == nil {
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error().Err(err).Msg("check admin user")
		return
	}
	password := cfg.Password
	user := &model.User{
		Username: cfg.Username, Email: cfg.Email, IsAdmin: true, Provider: "local", ProviderID: cfg.Username,
	}
	if err := user.SetPassword(password); err != nil {
		log.Error().Err(err).Msg("hash admin password")
		return
	}
	if err := userStore.Create(context.Background(), user); err != nil {
		if store.IsUniqueViolation(err) {
			log.Info().Str("username", cfg.Username).Msg("admin user already exists")
		} else {
			log.Error().Err(err).Msg("seed admin")
		}
	} else {
		log.Info().Str("username", cfg.Username).Msg("admin user created")
	}
}

func mountOIDCHandler(app *fiber.App, h http.Handler) {
	adapted := adaptor.HTTPHandler(h)
	for _, p := range []string{
		"/.well-known/{path...}", "/authorize", "/authorize/callback",
		"/oauth/userinfo", "/oauth/keys",
		"/oauth/introspect", "/oauth/revoke", "/oauth/end_session",
	} {
		app.All(p, adapted)
	}
}

func mountSPA(app *fiber.App, webDistFS fs.FS) {
	subFS, err := fs.Sub(webDistFS, "web/dist")
	if err != nil {
		logger.L.Error().Err(err).Msg("embed sub fs")
		return
	}
	indexHTML, err := fs.ReadFile(subFS, "index.html")
	if err != nil {
		logger.L.Info().Msg("index.html not found in embedded fs, skipping frontend")
		return
	}
	assetsFS, err := fs.Sub(subFS, "assets")
	if err != nil {
		logger.L.Error().Err(err).Msg("embed assets sub fs")
		return
	}

	app.Use("/assets", static.New("", static.Config{
		FS: assetsFS,
		NotFoundHandler: func(c fiber.Ctx) error {
			return c.SendStatus(fiber.StatusNotFound)
		},
		MaxAge: int((365 * 24 * time.Hour) / time.Second),
	}))
	app.Use("/", static.New("", static.Config{
		FS: subFS,
		Next: func(c fiber.Ctx) bool {
			return c.Path() == "/" || c.Path() == "/index.html"
		},
	}))
	serveIndex := func(c fiber.Ctx) error {
		c.Set("Content-Type", "text/html")
		c.Set("Cache-Control", "no-store")
		return c.Send(indexHTML)
	}
	app.Get("/", serveIndex)
	app.Get("/index.html", serveIndex)
	app.Get("/*", func(c fiber.Ctx) error {
		// Don't serve index.html for API routes to avoid masking 404s
		path := c.Path()
		if len(path) >= 4 && path[:4] == "/api" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
		}
		// Never serve SPA fallback for hashed/static file requests.
		if strings.Contains(path, ".") {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return serveIndex(c)
	})
}

func runServer(app *fiber.App, resources runtimeResources, addr string, buildInfo BuildInfo, log *zerolog.Logger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info().Str("addr", addr).Str("version", buildInfo.Version).Str("commit", buildInfo.GitCommit).Msg("starting server")
		if err := app.Listen(addr); err != nil {
			log.Error().Err(err).Msg("server error")
		}
	}()

	<-quit
	log.Info().Msg("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = app.ShutdownWithContext(shutdownCtx)
	resources.close(log)
}

func (r runtimeResources) close(log *zerolog.Logger) {
	if err := r.sessionStorage.Close(); err != nil && log != nil {
		log.Error().Err(err).Msg("close session storage")
	}
	if r.cache != nil {
		if err := r.cache.Close(); err != nil && log != nil {
			log.Error().Err(err).Msg("close cache")
		}
	}
	if r.db != nil {
		sqlDB, err := r.db.DB()
		if err != nil {
			if log != nil {
				log.Error().Err(err).Msg("resolve sql db")
			}
			return
		}
		if err := sqlDB.Close(); err != nil && log != nil {
			log.Error().Err(err).Msg("close db")
		}
	}
}

func cleanupAPIRefreshTokens(ctx context.Context, log *zerolog.Logger, s *store.APIRefreshTokenStore) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if err := s.DeleteExpired(ctx); err != nil {
			log.Error().Err(err).Msg("cleanup expired api refresh tokens")
		}
	}
}
