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

// ResolveAuditSource determines the canonical source for a request.
func ResolveAuditSource(c fiber.Ctx) string {
	if c == nil {
		return model.AuditSourceSystem
	}

	if headerSource := normalizeAuditSource(c.Get("X-Audit-Source")); headerSource != "" {
		return headerSource
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

func normalizeAuditSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case model.AuditSourceWeb:
		return model.AuditSourceWeb
	case model.AuditSourceAPI:
		return model.AuditSourceAPI
	case model.AuditSourceAdmin:
		return model.AuditSourceAdmin
	case model.AuditSourceCLI:
		return model.AuditSourceCLI
	case model.AuditSourceSystem:
		return model.AuditSourceSystem
	default:
		return ""
	}
}
