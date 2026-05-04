package response

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestFinishHandlerErrorWritesJSONError(t *testing.T) {
	app := fiber.New()
	app.Get("/error", func(c fiber.Ctx) error {
		return FinishHandlerError(c, JSONError(fiber.StatusBadRequest, "invalid input"))
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/error", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
	body := string(readResponseBody(t, resp.Body))
	if !strings.Contains(body, `"error":"invalid input"`) {
		t.Fatalf("expected error body, got %s", body)
	}
}
