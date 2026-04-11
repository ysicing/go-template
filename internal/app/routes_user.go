package app

import (
	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/store"
)

func registerUserModule(api fiber.Router, h *builtHandlers, jwtMW, tokenVersionMW, emailVerified fiber.Handler, cache store.Cache) {
	users := api.Group("/users", jwtMW, tokenVersionMW)
	users.Get("/me", h.user.GetMe)
	users.Put("/me", h.user.UpdateMe)
	users.Put("/me/password", h.user.ChangePassword)
	users.Post("/me/set-password", h.user.SetPassword)
	users.Get("/me/login-history", h.user.GetLoginHistory)
	users.Get("/me/authorized-apps", h.user.ListAuthorizedApps)
	users.Delete("/me/authorized-apps/:id", h.user.RevokeAuthorizedApp)
	users.Get("/me/social-accounts", h.socialAcct.ListMySocialAccounts)
	users.Delete("/me/social-accounts/:id", h.socialAcct.UnlinkSocialAccount)

	sessions := api.Group("/sessions", jwtMW, tokenVersionMW)
	sessions.Get("/", h.user.ListSessions)
	sessions.Delete("/:id", h.user.RevokeSession)
	sessions.Delete("/", h.user.RevokeAllSessions)

	mfa := api.Group("/mfa", jwtMW, tokenVersionMW)
	mfa.Get("/status", h.mfa.Status)
	mfa.Post("/totp/setup", h.mfa.TOTPSetup)
	mfa.Post("/totp/enable", h.mfa.TOTPEnable)
	mfa.Post("/totp/disable", h.mfa.TOTPDisable)
	mfa.Post("/backup-codes/regenerate", h.mfa.RegenerateBackupCodes)

	wa := api.Group("/mfa/webauthn", jwtMW, tokenVersionMW)
	wa.Get("/credentials", h.webauthn.ListCredentials)
	wa.Post("/register/begin", h.webauthn.RegisterBegin)
	wa.Post("/register/finish", h.webauthn.RegisterFinish)
	wa.Delete("/credentials/:id", h.webauthn.DeleteCredential)

	apps := api.Group("/apps", jwtMW, tokenVersionMW)
	apps.Get("/stats", h.app.Stats)
	appsV := apps.Use(emailVerified)
	appsV.Post("/", h.app.Create)
	appsV.Get("/", h.app.List)
	appsV.Get("/:id", h.app.Get)
	appsV.Put("/:id", h.app.Update)
	appsV.Delete("/:id", h.app.Delete)
	appsV.Post("/:id/rotate-secret", h.app.RotateSecret)
}
