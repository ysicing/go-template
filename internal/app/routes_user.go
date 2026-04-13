package app

import "github.com/gofiber/fiber/v3"

func userRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	routes := make([]managedRouteSpec, 0)
	routes = append(routes, currentUserRouteSpecs(rt)...)
	routes = append(routes, sessionRouteSpecs(rt)...)
	routes = append(routes, mfaRouteSpecs(rt)...)
	return routes
}

func currentUserRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/users/me", Summary: "Get current user", Tag: "user", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.GetMe}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPut, Path: "/api/users/me", Summary: "Update current user", Tag: "user", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.UpdateMe}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPut, Path: "/api/users/me/password", Summary: "Change password", Tag: "user", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.ChangePassword}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/users/me/set-password", Summary: "Set password", Tag: "user", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.SetPassword}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/users/me/login-history", Summary: "Get login history", Tag: "user", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.GetLoginHistory}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/users/me/authorized-apps", Summary: "List authorized apps", Tag: "user", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.ListAuthorizedApps}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodDelete, Path: "/api/users/me/authorized-apps/:id", Summary: "Revoke authorized app", Tag: "user", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.RevokeAuthorizedApp}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/users/me/social-accounts", Summary: "List linked social accounts", Tag: "user", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.socialAcct.ListMySocialAccounts}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodDelete, Path: "/api/users/me/social-accounts/:id", Summary: "Unlink social account", Tag: "user", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.socialAcct.UnlinkSocialAccount}
		}},
	}
}

func sessionRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/sessions", Summary: "List user sessions", Tag: "session", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.ListSessions}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodDelete, Path: "/api/sessions/:id", Summary: "Revoke one session", Tag: "session", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.RevokeSession}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodDelete, Path: "/api/sessions", Summary: "Revoke all sessions", Tag: "session", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.user.RevokeAllSessions}
		}},
	}
}

func mfaRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/mfa/status", Summary: "Get MFA status", Tag: "mfa", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.mfa.Status}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/mfa/totp/setup", Summary: "Setup TOTP", Tag: "mfa", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.mfa.TOTPSetup}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/mfa/totp/enable", Summary: "Enable TOTP", Tag: "mfa", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.mfa.TOTPEnable}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/mfa/totp/disable", Summary: "Disable TOTP", Tag: "mfa", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.mfa.TOTPDisable}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/mfa/backup-codes/regenerate", Summary: "Regenerate backup codes", Tag: "mfa", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.mfa.RegenerateBackupCodes}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/mfa/webauthn/credentials", Summary: "List WebAuthn credentials", Tag: "mfa", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.webauthn.ListCredentials}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/mfa/webauthn/register/begin", Summary: "Begin WebAuthn registration", Tag: "mfa", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.webauthn.RegisterBegin}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/mfa/webauthn/register/finish", Summary: "Finish WebAuthn registration", Tag: "mfa", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.webauthn.RegisterFinish}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodDelete, Path: "/api/mfa/webauthn/credentials/:id", Summary: "Delete WebAuthn credential", Tag: "mfa", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.jwtMW, rt.tokenVersionMW, rt.handlers.webauthn.DeleteCredential}
		}},
	}
}
