package bootstrap

import (
	"context"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/cache"
	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
	"github.com/ysicing/go-template/internal/httpserver"
	"github.com/ysicing/go-template/internal/setup"
	"github.com/ysicing/go-template/internal/user"
)

func Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	app, host, port, err := buildApp(cfg)
	if err != nil {
		return err
	}

	return app.Listen(fmt.Sprintf("%s:%d", host, port))
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(config.DefaultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return cfg, nil
}

func buildApp(cfg *config.Config) (*fiber.App, string, int, error) {
	host := "0.0.0.0"
	port := 8080
	if cfg == nil {
		app := httpserver.New(httpserver.Dependencies{
			Tokens:        auth.NewTokenManager("go-template", "change-me", config.Duration("15m").Value(), config.Duration("168h").Value()),
			Setup:         setup.NewService(config.DefaultPath, nil),
			SetupRequired: true,
		})
		return app, host, port, nil
	}

	conn, err := db.Open(cfg.Database)
	if err != nil {
		return nil, "", 0, err
	}
	if err := db.AutoMigrate(conn); err != nil {
		return nil, "", 0, err
	}

	setupService := setup.NewService(config.DefaultPath, conn)
	setupRequired, err := setupService.SetupRequired()
	if err != nil {
		return nil, "", 0, err
	}

	store, err := cache.NewStore(context.Background(), cfg.Cache)
	if err != nil {
		return nil, "", 0, err
	}
	_ = store

	tokens := auth.NewTokenManager(
		cfg.JWT.Issuer,
		cfg.JWT.Secret,
		cfg.JWT.AccessTTL.Value(),
		cfg.JWT.RefreshTTL.Value(),
	)

	app := httpserver.New(httpserver.Dependencies{
		DB:            conn,
		UserService:   user.NewService(conn),
		Auth:          auth.NewService(conn, tokens),
		Tokens:        tokens,
		Setup:         setupService,
		SetupRequired: setupRequired,
	})

	return app, cfg.Server.Host, cfg.Server.Port, nil
}
