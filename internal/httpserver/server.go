package httpserver

import (
	"context"
	"io/fs"
	"mime"
	"path/filepath"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/ysicing/go-template/internal/auth"
	"github.com/ysicing/go-template/internal/cache"
	"github.com/ysicing/go-template/internal/config"
	"github.com/ysicing/go-template/internal/db"
	"github.com/ysicing/go-template/internal/setup"
	"github.com/ysicing/go-template/internal/user"
	"github.com/ysicing/go-template/web"
	"gorm.io/gorm"
)

type Dependencies struct {
	DB            *gorm.DB
	UserService   *user.Service
	Auth          *auth.Service
	Tokens        *auth.TokenManager
	Setup         *setup.Service
	SetupRequired bool
}

type State struct {
	mu            sync.RWMutex
	db            *gorm.DB
	userService   *user.Service
	auth          *auth.Service
	tokens        *auth.TokenManager
	setup         *setup.Service
	setupRequired bool
}

func New(deps Dependencies) *fiber.App {
	app := fiber.New()
	state := newState(deps)
	registerRoutes(app, state)
	registerStatic(app)
	return app
}

func NewForTest(setupRequired bool) *fiber.App {
	tokens := auth.NewTokenManager("issuer", "secret", config.Duration("15m").Value(), config.Duration("1h").Value())
	return New(Dependencies{
		Tokens:        tokens,
		Setup:         setup.NewService("configs/config.yaml", nil),
		SetupRequired: setupRequired,
	})
}

func newState(deps Dependencies) *State {
	return &State{
		db:            deps.DB,
		userService:   deps.UserService,
		auth:          deps.Auth,
		tokens:        deps.Tokens,
		setup:         deps.Setup,
		setupRequired: deps.SetupRequired,
	}
}

func (s *State) SetupRequired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.setupRequired
}

func (s *State) Setup() *setup.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.setup
}

func (s *State) Auth() *auth.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.auth
}

func (s *State) UserService() *user.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userService
}

func (s *State) Tokens() *auth.TokenManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tokens
}

func (s *State) DB() *gorm.DB {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db
}

func (s *State) ReloadFromConfig(path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	conn, err := db.Open(cfg.Database)
	if err != nil {
		return err
	}
	if err := db.AutoMigrate(conn); err != nil {
		return err
	}
	if _, err := cache.NewStore(context.Background(), cfg.Cache); err != nil {
		return err
	}

	setupService := setup.NewService(path, conn)
	setupRequired, err := setupService.SetupRequired()
	if err != nil {
		return err
	}

	tokens := auth.NewTokenManager(cfg.JWT.Issuer, cfg.JWT.Secret, cfg.JWT.AccessTTL.Value(), cfg.JWT.RefreshTTL.Value())
	userService := user.NewService(conn)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = conn
	s.userService = userService
	s.tokens = tokens
	s.auth = auth.NewService(conn, tokens)
	s.setup = setupService
	s.setupRequired = setupRequired
	return nil
}

func registerRoutes(app *fiber.App, state *State) {
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	registerSetupRoutes(app, state)
	registerAuthRoutes(app, state)
	registerAdminUserRoutes(app, state)
	registerSystemRoutes(app, state)
}

func registerStatic(app *fiber.App) {
	sub, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return
	}

	app.Use("/", static.New("", static.Config{FS: sub, Browse: false}))
	app.Get("/*", func(c fiber.Ctx) error {
		content, readErr := fs.ReadFile(sub, "index.html")
		if readErr != nil {
			return c.SendStatus(fiber.StatusNotFound)
		}

		c.Set("Content-Type", mime.TypeByExtension(filepath.Ext("index.html")))
		return c.Send(content)
	})
}
