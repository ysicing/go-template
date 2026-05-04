package audit

import (
	"strings"

	"github.com/ysicing/go-template/model"

	"github.com/gofiber/fiber/v3"
)

const auditSourceLocalKey = "audit_source"

// AuditContextMiddleware 为每个请求写入标准化的审计来源。
func AuditContextMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals(auditSourceLocalKey, ResolveAuditSource(c))
		return c.Next()
	}
}

// ResolveAuditSource 根据请求路径确定审计来源。
func ResolveAuditSource(c fiber.Ctx) string {
	if c == nil {
		return model.AuditSourceSystem
	}

	path := strings.ToLower(c.Path())
	switch {
	case strings.HasPrefix(path, "/api/admin"):
		return model.AuditSourceAdmin
	case strings.HasPrefix(path, "/api/"):
		return model.AuditSourceAPI
	default:
		return model.AuditSourceWeb
	}
}
