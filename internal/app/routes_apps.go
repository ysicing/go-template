package app

import "github.com/gofiber/fiber/v3"

func pointsRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{
			Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/points", Summary: "Get point balance", Tag: "points", RequiresAuth: true},
			Handlers: func(rt managedRouteRuntime) []fiber.Handler {
				return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.points.GetMyPoints}
			},
		},
		{
			Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/points/transactions", Summary: "List point transactions", Tag: "points", RequiresAuth: true},
			Handlers: func(rt managedRouteRuntime) []fiber.Handler {
				return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.points.GetTransactions}
			},
		},
		{
			Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/points/checkin/status", Summary: "Get checkin status", Tag: "points", RequiresAuth: true},
			Handlers: func(rt managedRouteRuntime) []fiber.Handler {
				return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.points.GetCheckInStatus}
			},
		},
		{
			Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/points/checkin", Summary: "Check in for points", Tag: "points", RequiresAuth: true},
			Handlers: func(rt managedRouteRuntime) []fiber.Handler {
				return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.emailVerified, rt.pointsLimiter, rt.handlers.points.CheckIn}
			},
		},
		{
			Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/points/spend", Summary: "Spend points", Tag: "points", RequiresAuth: true},
			Handlers: func(rt managedRouteRuntime) []fiber.Handler {
				return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.emailVerified, rt.pointsLimiter, rt.handlers.points.SpendPoints}
			},
		},
	}
}
