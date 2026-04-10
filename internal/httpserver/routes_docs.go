package httpserver

import (
	_ "github.com/ysicing/go-template/internal/apidocs"

	fiberswagger "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
)

func registerDocsRoutes(app *fiber.App) {
	app.Get("/swagger/*", fiberswagger.New(fiberswagger.Config{
		Title:       "go-template API Docs",
		DeepLinking: true,
	}))
}
