package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/middleware"
)

func writeError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message})
}

// userIDFromCtx recupera o ID do usuário autenticado injetado pelo middleware de auth.
func userIDFromCtx(c *fiber.Ctx) string {
	if v, ok := c.Locals(middleware.LocalsUserID).(string); ok {
		return v
	}
	return ""
}
