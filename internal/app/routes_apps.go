package app

import (
	"github.com/gofiber/fiber/v3"
)

func registerAppsModule(api fiber.Router, h *builtHandlers, jwtMW, tokenVersionMW, emailVerified, pointsLimiter fiber.Handler) {
	registerPointsModule(api, h, jwtMW, tokenVersionMW, emailVerified, pointsLimiter)
}

func registerPointsModule(api fiber.Router, h *builtHandlers, jwtMW, tokenVersionMW, emailVerified, pointsLimiter fiber.Handler) {
	points := api.Group("/points", jwtMW, tokenVersionMW)
	points.Get("/", h.points.GetMyPoints)
	points.Get("/transactions", h.points.GetTransactions)
	points.Get("/checkin/status", h.points.GetCheckInStatus)
	points.Post("/checkin", emailVerified, pointsLimiter, h.points.CheckIn)
	points.Post("/spend", emailVerified, pointsLimiter, h.points.SpendPoints)
}
