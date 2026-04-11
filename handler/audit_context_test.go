package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/model"
)

func TestResolveAuditSource(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		header   string
		expected string
	}{
		{name: "admin path", path: "/api/admin/users", expected: model.AuditSourceAdmin},
		{name: "api path", path: "/api/users/me", expected: model.AuditSourceAPI},
		{name: "web path", path: "/profile", expected: model.AuditSourceWeb},
		{name: "header override", path: "/profile", header: model.AuditSourceCLI, expected: model.AuditSourceCLI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(AuditContextMiddleware())
			app.Get("/*", func(c fiber.Ctx) error {
				got, _ := c.Locals(auditSourceLocalKey).(string)
				if got != tt.expected {
					t.Fatalf("expected %s, got %s", tt.expected, got)
				}
				return c.SendStatus(fiber.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.header != "" {
				req.Header.Set("X-Audit-Source", tt.header)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != fiber.StatusNoContent {
				t.Fatalf("expected 204, got %d", resp.StatusCode)
			}
		})
	}
}
