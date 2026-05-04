package clientcredentialshandler

import (
	"encoding/base64"
	"strings"

	clientcredentialsservice "github.com/ysicing/go-template/internal/service/clientcredentials"

	"github.com/gofiber/fiber/v3"
)

type ClientCredentialsHandler struct {
	service *clientcredentialsservice.ClientCredentialsService
}

func NewClientCredentialsHandler(service *clientcredentialsservice.ClientCredentialsService) *ClientCredentialsHandler {
	return &ClientCredentialsHandler{service: service}
}

func (h *ClientCredentialsHandler) Token(c fiber.Ctx) error {
	if strings.TrimSpace(c.FormValue("grant_type")) != "client_credentials" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported_grant_type"})
	}

	clientID, clientSecret := extractOAuthClientCredentials(c)
	resp, err := h.service.Exchange(c.Context(), clientcredentialsservice.ClientCredentialsExchangeInput{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		GrantType:    "client_credentials",
		Scope:        strings.TrimSpace(c.FormValue("scope")),
	})
	if err != nil {
		switch err {
		case clientcredentialsservice.ErrClientCredentialsInvalidClient:
			c.Set("WWW-Authenticate", `Basic realm="oauth"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_client"})
		case clientcredentialsservice.ErrClientCredentialsUnauthorizedClient:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unauthorized_client"})
		case clientcredentialsservice.ErrClientCredentialsInvalidScope:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_scope"})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server_error"})
		}
	}

	return c.JSON(resp)
}

func (h *ClientCredentialsHandler) Introspect(c fiber.Ctx) error {
	clientID, clientSecret := extractOAuthClientCredentials(c)
	resp, handled, err := h.service.Introspect(c.Context(), clientcredentialsservice.ClientCredentialsIntrospectionInput{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Token:        strings.TrimSpace(c.FormValue("token")),
	})
	if err != nil {
		if err == clientcredentialsservice.ErrClientCredentialsInvalidClient {
			c.Set("WWW-Authenticate", `Basic realm="oauth"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_client"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server_error"})
	}
	if handled {
		return c.JSON(resp)
	}
	return c.JSON(&clientcredentialsservice.ClientCredentialsIntrospectionResponse{})
}

func (h *ClientCredentialsHandler) Revoke(c fiber.Ctx) error {
	clientID, clientSecret := extractOAuthClientCredentials(c)
	handled, err := h.service.Revoke(c.Context(), clientcredentialsservice.ClientCredentialsRevokeInput{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Token:        strings.TrimSpace(c.FormValue("token")),
	})
	if err != nil {
		if err == clientcredentialsservice.ErrClientCredentialsInvalidClient {
			c.Set("WWW-Authenticate", `Basic realm="oauth"`)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_client"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server_error"})
	}
	if handled {
		return c.SendStatus(fiber.StatusOK)
	}
	return c.SendStatus(fiber.StatusOK)
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
