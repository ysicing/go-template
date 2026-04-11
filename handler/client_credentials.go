package handler

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/ysicing/go-template/internal/service"
)

type ClientCredentialsHandler struct {
	service  *service.ClientCredentialsService
	fallback http.Handler
}

func NewClientCredentialsHandler(service *service.ClientCredentialsService, fallback http.Handler) *ClientCredentialsHandler {
	return &ClientCredentialsHandler{service: service, fallback: fallback}
}

func (h *ClientCredentialsHandler) Token(c fiber.Ctx) error {
	if strings.TrimSpace(c.FormValue("grant_type")) != "client_credentials" {
		return h.delegate(c, "/oauth/token")
	}

	clientID, clientSecret := extractOAuthClientCredentials(c)
	resp, err := h.service.Exchange(c.Context(), service.ClientCredentialsExchangeInput{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		GrantType:    "client_credentials",
		Scope:        strings.TrimSpace(c.FormValue("scope")),
	})
	if err != nil {
		switch err {
		case service.ErrClientCredentialsInvalidClient:
			c.Set("WWW-Authenticate", `Basic realm="oauth"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_client"})
		case service.ErrClientCredentialsUnauthorizedClient:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unauthorized_client"})
		case service.ErrClientCredentialsInvalidScope:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_scope"})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server_error"})
		}
	}

	return c.JSON(resp)
}

func (h *ClientCredentialsHandler) Introspect(c fiber.Ctx) error {
	clientID, clientSecret := extractOAuthClientCredentials(c)
	resp, handled, err := h.service.Introspect(c.Context(), service.ClientCredentialsIntrospectionInput{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Token:        strings.TrimSpace(c.FormValue("token")),
	})
	if err != nil {
		if err == service.ErrClientCredentialsInvalidClient {
			c.Set("WWW-Authenticate", `Basic realm="oauth"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_client"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server_error"})
	}
	if handled {
		return c.JSON(resp)
	}
	return h.delegate(c, "/oauth/introspect")
}

func (h *ClientCredentialsHandler) Revoke(c fiber.Ctx) error {
	clientID, clientSecret := extractOAuthClientCredentials(c)
	handled, err := h.service.Revoke(c.Context(), service.ClientCredentialsRevokeInput{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Token:        strings.TrimSpace(c.FormValue("token")),
	})
	if err != nil {
		if err == service.ErrClientCredentialsInvalidClient {
			c.Set("WWW-Authenticate", `Basic realm="oauth"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_client"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server_error"})
	}
	if handled {
		return c.SendStatus(fiber.StatusOK)
	}
	return h.delegate(c, "/oauth/revoke")
}

func (h *ClientCredentialsHandler) delegate(c fiber.Ctx, path string) error {
	if h.fallback == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}

	req := httptest.NewRequest(c.Method(), path, strings.NewReader(string(c.Body())))
	req.Header.Set("Content-Type", c.Get("Content-Type"))
	if accept := c.Get("Accept"); accept != "" {
		req.Header.Set("Accept", accept)
	}
	if auth := c.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Host = c.Hostname()
	if fh := c.Get("X-Forwarded-Host"); fh != "" {
		req.Header.Set("X-Forwarded-Host", fh)
	}
	if fp := c.Get("X-Forwarded-Proto"); fp != "" {
		req.Header.Set("X-Forwarded-Proto", fp)
	}

	rec := httptest.NewRecorder()
	h.fallback.ServeHTTP(rec, req)

	c.Set("Content-Type", rec.Header().Get("Content-Type"))
	return c.Status(rec.Code).Send(rec.Body.Bytes())
}

func extractOAuthClientCredentials(c fiber.Ctx) (string, string) {
	auth := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(auth, "Basic ") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(auth, "Basic ")))
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			}
		}
	}
	return strings.TrimSpace(c.FormValue("client_id")), strings.TrimSpace(c.FormValue("client_secret"))
}
