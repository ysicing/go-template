package handler

import (
	"github.com/ysicing/go-template/model"

	"github.com/gofiber/fiber/v3"
)

type emailVerificationSender interface {
	SendVerificationEmail(c fiber.Ctx, user *model.User, baseURL string) error
}
