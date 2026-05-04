package request

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestParsePagination_DefaultsAndBounds(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantPage     int
		wantPageSize int
	}{
		{name: "defaults", path: "/items", wantPage: 1, wantPageSize: 20},
		{name: "valid", path: "/items?page=3&page_size=50", wantPage: 3, wantPageSize: 50},
		{name: "lower bounds", path: "/items?page=0&page_size=0", wantPage: 1, wantPageSize: 20},
		{name: "upper bound", path: "/items?page=2&page_size=500", wantPage: 2, wantPageSize: 100},
		{name: "invalid numbers", path: "/items?page=bad&page_size=bad", wantPage: 1, wantPageSize: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/items", func(c fiber.Ctx) error {
				page, pageSize := ParsePagination(c)
				if page != tt.wantPage || pageSize != tt.wantPageSize {
					t.Fatalf("expected page=%d pageSize=%d, got page=%d pageSize=%d", tt.wantPage, tt.wantPageSize, page, pageSize)
				}
				return c.SendStatus(fiber.StatusNoContent)
			})

			resp, err := app.Test(httptest.NewRequest("GET", tt.path, nil))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.StatusCode != fiber.StatusNoContent {
				t.Fatalf("expected 204, got %d", resp.StatusCode)
			}
		})
	}
}
