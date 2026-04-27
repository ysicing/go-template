package handler

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

type responseError struct {
	status int
	body   fiber.Map
}

func (e *responseError) Error() string {
	if msg, ok := e.body["error"].(string); ok {
		return msg
	}
	return "response error"
}

func jsonError(status int, msg string) error {
	return &responseError{
		status: status,
		body:   fiber.Map{"error": msg},
	}
}

func finishHandlerError(c fiber.Ctx, err error) error {
	var respErr *responseError
	if errors.As(err, &respErr) {
		return c.Status(respErr.status).JSON(respErr.body)
	}
	return err
}

func JSONError(status int, msg string) error {
	return jsonError(status, msg)
}

func FinishHandlerError(c fiber.Ctx, err error) error {
	return finishHandlerError(c, err)
}
