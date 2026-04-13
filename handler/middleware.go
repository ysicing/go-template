package handler

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"
)

// Claims defines the JWT token payload.
type Claims struct {
	UserID       string   `json:"user_id"`
	IsAdmin      bool     `json:"is_admin"`
	Permissions  []string `json:"permissions,omitempty"`
	TokenVersion int64    `json:"token_version"`
	TokenType    string   `json:"token_type"`
	jwt.RegisteredClaims
}

// JWTMiddleware validates Bearer token from Authorization header or Cookie.
// Supports both authentication methods:
// 1. Authorization: Bearer <token>
// 2. Cookie: access_token=<token>
func JWTMiddleware(secret, issuer string) fiber.Handler {
	return func(c fiber.Ctx) error {
		tokenStr := bearerToken(c)
		if tokenStr == "" {
			tokenStr = c.Cookies("access_token")
		}
		if tokenStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing or invalid token"})
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		if claims.TokenType != "access" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token type"})
		}
		if claims.Issuer != issuer {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token issuer"})
		}
		if claims.TokenVersion < 1 {
			claims.TokenVersion = 1
		}

		c.Locals("principal_type", "user")
		c.Locals("user_id", claims.UserID)
		c.Locals("is_admin", claims.IsAdmin)
		c.Locals("permissions", claims.Permissions)
		c.Locals("token_version", claims.TokenVersion)
		return c.Next()
	}
}

func bearerToken(c fiber.Ctx) string {
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
}

// TokenVersionMiddleware validates token_version claim against current user version.
// Must be used after JWTMiddleware.
func TokenVersionMiddleware(users *store.UserStore, cache store.Cache) fiber.Handler {
	return func(c fiber.Ctx) error {
		if principalType, _ := c.Locals("principal_type").(string); principalType == "service_account" {
			return c.Next()
		}
		userID, _ := c.Locals("user_id").(string)
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		tokenVersion, _ := c.Locals("token_version").(int64)
		if tokenVersion < 1 {
			tokenVersion = 1
		}
		currentVersion, err := loadCurrentTokenVersion(c.Context(), users, cache, userID)
		if err != nil || tokenVersion != currentVersion {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}
		return c.Next()
	}
}

// AdminMiddleware checks if user is admin from JWT claims.
// Must be used after JWTMiddleware.
// Uses 30-second cache to reduce database load.
func AdminMiddleware(users *store.UserStore, cache store.Cache) fiber.Handler {
	return RequirePermission(users, cache, model.PermissionAdminStatsRead)
}

func userHasPermission(user *model.User, permission string) bool {
	if permission == "" {
		return true
	}
	if user.HasPermission(permission) {
		return true
	}
	return user.IsAdmin
}

func loadCurrentTokenVersion(ctx context.Context, users *store.UserStore, cache store.Cache, userID string) (int64, error) {
	cacheKey := "token_ver:" + userID
	if cached, err := cache.Get(ctx, cacheKey); err == nil {
		ver, parseErr := strconv.ParseInt(cached, 10, 64)
		if parseErr == nil {
			if ver < 1 {
				return 1, nil
			}
			return ver, nil
		}
	}

	user, err := users.GetByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	ver := user.TokenVersion
	if ver < 1 {
		ver = 1
	}
	_ = cache.Set(ctx, cacheKey, strconv.FormatInt(ver, 10), 30*time.Second)
	return ver, nil
}

// RequirePermission checks user has required permission from JWT claims or DB fallback.
func RequirePermission(users *store.UserStore, cache store.Cache, permission string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if permission == "" {
			return c.Next()
		}

		userID, _ := c.Locals("user_id").(string)
		if userID == "" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
		}
		tokenVersion, _ := c.Locals("token_version").(int64)
		if tokenVersion < 1 {
			tokenVersion = 1
		}
		currentVersion, err := loadCurrentTokenVersion(c.Context(), users, cache, userID)
		if err != nil || tokenVersion != currentVersion {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
		}

		if claimsPerms, ok := c.Locals("permissions").([]string); ok && len(claimsPerms) > 0 {
			if slices.Contains(claimsPerms, permission) {
				return c.Next()
			}
		}
		isAdmin, _ := c.Locals("is_admin").(bool)
		if isAdmin {
			return c.Next()
		}

		cacheKey := "perm_check:" + userID + ":" + permission
		if cached, err := cache.Get(c.Context(), cacheKey); err == nil {
			if cached == "1" {
				return c.Next()
			}
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
		}

		user, err := users.GetByID(c.Context(), userID)
		if err != nil || !userHasPermission(user, permission) {
			_ = cache.Set(c.Context(), cacheKey, "0", 10*time.Second)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "permission denied"})
		}

		_ = cache.Set(c.Context(), cacheKey, "1", 30*time.Second)
		return c.Next()
	}
}

// EmailVerifiedMiddleware blocks unverified users when email verification is enabled.
// Must be used after JWTMiddleware.
// Uses short-lived cache to reduce database load on protected routes.
func EmailVerifiedMiddleware(users *store.UserStore, settings *store.SettingStore, cache store.Cache) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !settings.GetBool(store.SettingEmailVerificationEnabled, false) {
			return c.Next()
		}
		userID, _ := c.Locals("user_id").(string)
		if userID == "" {
			return c.Next()
		}

		cacheKey := "email_verified:" + userID
		if cached, err := cache.Get(c.Context(), cacheKey); err == nil {
			if cached == "1" {
				return c.Next()
			}
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "email_not_verified"})
		}

		user, err := users.GetByID(c.Context(), userID)
		if err != nil || !user.EmailVerified {
			_ = cache.Set(c.Context(), cacheKey, "0", 10*time.Second)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "email_not_verified"})
		}

		_ = cache.Set(c.Context(), cacheKey, "1", 30*time.Second)
		return c.Next()
	}
}

// GenerateAccessToken creates a standalone access token for tests and narrow helper flows.
func GenerateAccessToken(userID string, isAdmin bool, permissions []string, tokenVersion int64, secret, issuer string, accessTTL time.Duration) (string, error) {
	now := time.Now()
	if tokenVersion < 1 {
		tokenVersion = 1
	}
	claims := Claims{
		UserID:       userID,
		IsAdmin:      isAdmin,
		Permissions:  permissions,
		TokenVersion: tokenVersion,
		TokenType:    "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID,
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{"id-api"},
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

// RequestIDMiddleware ensures every request has a unique X-Request-ID header.
// If the client provides one, it is reused; otherwise a new UUID is generated.
// The ID is also stored in Locals("request_id") for structured logging.
func RequestIDMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		reqID := c.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Set("X-Request-ID", reqID)
		c.Locals("request_id", reqID)
		return c.Next()
	}
}

// parsePagination extracts and validates page/pageSize from query parameters.
func parsePagination(c fiber.Ctx) (page, pageSize int) {
	page, _ = strconv.Atoi(c.Query("page", "1"))
	pageSize, _ = strconv.Atoi(c.Query("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// SetTokenCookies sets access and refresh tokens as HttpOnly cookies.
// This provides better security against XSS attacks compared to localStorage.
func SetTokenCookies(c fiber.Ctx, accessToken, refreshToken string, accessTTL, refreshTTL time.Duration) {
	// Set access token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		MaxAge:   int(accessTTL.Seconds()),
		HTTPOnly: true,                  // Prevents JavaScript access (XSS protection)
		Secure:   c.Scheme() == "https", // Only send over HTTPS in production
		SameSite: "Lax",                 // CSRF protection
	})

	// Set refresh token cookie
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/api/auth/", // Send to auth endpoints (refresh + logout)
		MaxAge:   int(refreshTTL.Seconds()),
		HTTPOnly: true,
		Secure:   c.Scheme() == "https",
		SameSite: "Lax",
	})
}

// ClearTokenCookies removes authentication cookies on logout.
func ClearTokenCookies(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HTTPOnly: true,
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth/",
		MaxAge:   -1,
		HTTPOnly: true,
	})
}

// OptionalJWTMiddleware validates JWT token if present, but allows requests without token.
// Sets user_id in locals if token is valid, otherwise continues without authentication.
func OptionalJWTMiddleware(secret, issuer string) fiber.Handler {
	return func(c fiber.Ctx) error {
		var tokenStr string

		// Try to get token from Authorization header first
		auth := c.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		}

		// If not found in header, try to get from cookie
		if tokenStr == "" {
			tokenStr = c.Cookies("access_token")
		}

		// If no token found, continue without authentication
		if tokenStr == "" {
			return c.Next()
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		// If token is valid, set user info in locals
		if err == nil && token.Valid && claims.TokenType == "access" && claims.Issuer == issuer {
			c.Locals("user_id", claims.UserID)
			c.Locals("is_admin", claims.IsAdmin)
			c.Locals("permissions", claims.Permissions)
		}

		return c.Next()
	}
}
