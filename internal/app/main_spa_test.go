package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
)

func testWebDistFS() fstest.MapFS {
	return fstest.MapFS{
		"web/dist/index.html":    {Data: []byte("<!doctype html><html><body>app</body></html>")},
		"web/dist/assets/app.js": {Data: []byte("console.log('ok')")},
	}
}

func TestMountSPA_MissingAssetPathReturnsNotFound(t *testing.T) {
	fiberApp := fiber.New()
	mountSPA(fiberApp, testWebDistFS())

	req := httptest.NewRequest(http.MethodGet, "/assets/missing-style.css", nil)
	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404 for missing asset, got %d", resp.StatusCode)
	}
}

func TestMountSPA_ClientRouteSetsNoStoreCacheControl(t *testing.T) {
	fiberApp := fiber.New()
	mountSPA(fiberApp, testWebDistFS())

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for SPA route, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected Cache-Control no-store for SPA route, got %q", got)
	}
}

func TestMountSPA_RootSetsNoStoreCacheControl(t *testing.T) {
	fiberApp := fiber.New()
	mountSPA(fiberApp, testWebDistFS())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := fiberApp.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for root route, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected Cache-Control no-store for root route, got %q", got)
	}
}
