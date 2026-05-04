package middleware

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ysicing/go-template/model"
	"github.com/ysicing/go-template/store"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims 定义访问令牌中的 JWT 载荷。
type Claims struct {
	UserID       string   `json:"user_id"`
	IsAdmin      bool     `json:"is_admin"`
	Permissions  []string `json:"permissions,omitempty"`
	TokenVersion int64    `json:"token_version"`
	TokenType    string   `json:"token_type"`
	jwt.RegisteredClaims
}

// JWTMiddleware 校验 Authorization Bearer 或 access_token Cookie 中的访问令牌。
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

// TokenVersionMiddleware 校验令牌版本是否仍匹配当前用户版本，必须在 JWTMiddleware 之后使用。
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

// AdminMiddleware 校验当前用户是否拥有管理员统计读取权限，必须在 JWTMiddleware 之后使用。
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

// RequirePermission 校验当前用户是否拥有指定权限，优先使用 JWT 声明并在必要时回查数据库。
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

// EmailVerifiedMiddleware 在启用邮箱验证时阻止未验证用户访问受保护路由。
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

// GenerateAccessToken 生成独立访问令牌，主要用于测试和少量辅助流程。
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

// RequestIDMiddleware 确保每个请求都有 X-Request-ID，并同步写入 Locals("request_id")。
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

// OptionalJWTMiddleware 在令牌存在时尝试校验，缺失或无效时仍允许匿名继续访问。
func OptionalJWTMiddleware(secret, issuer string) fiber.Handler {
	return func(c fiber.Ctx) error {
		var tokenStr string

		// 优先从 Authorization 头读取 Bearer 令牌。
		auth := c.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			tokenStr = strings.TrimPrefix(auth, "Bearer ")
		}

		// 未提供头部令牌时再回退到 Cookie。
		if tokenStr == "" {
			tokenStr = c.Cookies("access_token")
		}

		// 未提供令牌时按匿名请求继续。
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

		// 令牌有效时才向 Locals 写入当前用户信息。
		if err == nil && token.Valid && claims.TokenType == "access" && claims.Issuer == issuer {
			c.Locals("user_id", claims.UserID)
			c.Locals("is_admin", claims.IsAdmin)
			c.Locals("permissions", claims.Permissions)
		}

		return c.Next()
	}
}
