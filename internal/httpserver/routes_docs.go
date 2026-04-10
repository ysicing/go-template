package httpserver

import (
	fiberswagger "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/ysicing/go-template/internal/apidocs"
	"github.com/ysicing/go-template/internal/buildinfo"
)

func registerDocsRoutes(app *fiber.App) {
	apidocs.SwaggerInfo.Version = buildinfo.FullVersion()
	app.Get("/swagger/*", fiberswagger.New(fiberswagger.Config{
		Title:       "go-template API Docs",
		DeepLinking: true,
	}))
}
