package app

import (
	"slices"
	"sync"

	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/rs/zerolog"
)

func buildCORSMiddleware(settingStore *store.SettingStore, log *zerolog.Logger) fiber.Handler {
	var (
		corsWarnOnce     sync.Once
		wildcardWarnOnce sync.Once
	)

	allowOriginsFunc := func(origin string) bool {
		if origin == "" {
			return true
		}

		origins := settingStore.GetStringSlice(store.SettingCORSOrigins, nil)
		if len(origins) == 0 {
			corsWarnOnce.Do(func() {
				log.Warn().Msg("CORS: no origins configured, denying cross-origin requests")
			})
			return false
		}

		for _, allowed := range origins {
			if allowed == "*" || allowed == origin {
				return true
			}
		}
		return false
	}

	withCredentials := cors.New(cors.Config{
		AllowOriginsFunc: allowOriginsFunc,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	})
	withoutCredentials := cors.New(cors.Config{
		AllowOriginsFunc: allowOriginsFunc,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: false,
	})

	return func(c fiber.Ctx) error {
		if slices.Contains(settingStore.GetStringSlice(store.SettingCORSOrigins, nil), "*") {
			wildcardWarnOnce.Do(func() {
				log.Warn().Msg("CORS: wildcard origin detected, credentials disabled for security")
			})
			return withoutCredentials(c)
		}
		return withCredentials(c)
	}
}
