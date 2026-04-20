package handler

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/model"
)

const auditSourceLocalKey = "audit_source"

// AuditContextMiddleware assigns a normalized audit source to each request.
func AuditContextMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals(auditSourceLocalKey, ResolveAuditSource(c))
		return c.Next()
	}
}

// ResolveAuditSource determines the canonical source for a request based on path.
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
