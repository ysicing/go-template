package app

import "github.com/gofiber/fiber/v3"

func authRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	routes := make([]managedRouteSpec, 0)
	routes = append(routes, authPublicRouteSpecs(rt)...)
	routes = append(routes, authOIDCRouteSpecs(rt)...)
	routes = append(routes, authMFARouteSpecs(rt)...)
	routes = append(routes, authSocialRouteSpecs(rt)...)
	routes = append(routes, authAccountRouteSpecs(rt)...)
	return routes
}

func authPublicRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/auth/config", Summary: "Read auth config", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.authConfigHandler}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/register", Summary: "Register local user", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.registerLimiter, rt.registerEnabledMW, rt.turnstile, rt.handlers.auth.Register}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/login", Summary: "Login with password", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.turnstile, rt.handlers.auth.Login}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/refresh", Summary: "Refresh tokens", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.auth.Refresh}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/verify-email", Summary: "Verify email token", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.email.VerifyEmail}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/setup-password", Summary: "Set password with one-time token", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.auth.SetupPassword}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/logout", Summary: "Logout current session", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.optionalJWT, rt.handlers.auth.Logout}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/resend-verification", Summary: "Resend verification email", Tag: "auth", RequiresAuth: true}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.jwtMW, rt.tokenVersionMW, rt.handlers.email.ResendVerification}
		}},
	}
}

func authOIDCRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/oidc-login", Summary: "Submit OIDC login", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.oidcLogin.LoginSubmit}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/auth/oidc/consent", Summary: "Read OIDC consent context", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.oidcLogin.ConsentContext}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/oidc/consent/approve", Summary: "Approve OIDC consent", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.oidcLogin.ConsentApprove}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/oidc/consent/deny", Summary: "Deny OIDC consent", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.oidcLogin.ConsentDeny}
		}},
	}
}

func authMFARouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/mfa/verify", Summary: "Verify MFA code", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.mfa.Verify}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/webauthn/begin", Summary: "Begin WebAuthn login", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.webauthn.LoginBegin}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/webauthn/finish", Summary: "Finish WebAuthn login", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.webauthn.LoginFinish}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/mfa/webauthn/begin", Summary: "Begin MFA WebAuthn", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.webauthn.AuthBegin}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/mfa/webauthn/finish", Summary: "Finish MFA WebAuthn", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.webauthn.AuthFinish}
		}},
	}
}

func authSocialRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/auth/github", Summary: "Start GitHub login", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.optionalJWT, rt.handlers.oauth.GitHubLogin}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/auth/github/callback", Summary: "Finish GitHub login", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.oauth.GitHubCallback}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/auth/google", Summary: "Start Google login", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.optionalJWT, rt.handlers.oauth.GoogleLogin}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/auth/google/callback", Summary: "Finish Google login", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.oauth.GoogleCallback}
		}},
	}
}

func authAccountRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/social/exchange", Summary: "Exchange social code", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.oauth.ExchangeCode}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/api/auth/social/confirm-link", Summary: "Confirm social account link", Tag: "auth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.authLimiter, rt.handlers.oauth.ConfirmSocialLink}
		}},
	}
}

func oauthRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/oauth/token", Summary: "OAuth token endpoint", Tag: "oauth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.handlers.clientCredentials.Token}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/oauth/introspect", Summary: "OAuth introspection endpoint", Tag: "oauth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.handlers.clientCredentials.Introspect}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/oauth/revoke", Summary: "OAuth revoke endpoint", Tag: "oauth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.handlers.clientCredentials.Revoke}
		}},
	}
}

func githubCompatRouteSpecs(rt managedRouteRuntime) []managedRouteSpec {
	return []managedRouteSpec{
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/login/oauth/authorize", Summary: "GitHub compatible authorize", Tag: "oauth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.ghLimiter, rt.handlers.ghCompat.Authorize}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodPost, Path: "/login/oauth/access_token", Summary: "GitHub compatible token", Tag: "oauth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.ghLimiter, rt.handlers.ghCompat.AccessToken}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/v3/user", Summary: "GitHub compatible current user", Tag: "oauth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.ghLimiter, rt.handlers.ghCompat.User}
		}},
		{Doc: openAPIRoute{Method: fiber.MethodGet, Path: "/api/v3/user/emails", Summary: "GitHub compatible user emails", Tag: "oauth"}, Handlers: func(rt managedRouteRuntime) []fiber.Handler {
			return []fiber.Handler{rt.ghLimiter, rt.handlers.ghCompat.UserEmails}
		}},
	}
}
