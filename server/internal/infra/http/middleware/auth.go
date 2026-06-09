package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// LocalsUserID é a chave usada para armazenar o ID do usuário autenticado em c.Locals.
const LocalsUserID = "userID"

// Auth valida o header Authorization: Bearer <token>, extrai o userID via
// TokenIssuer e o armazena em c.Locals. Retorna 401 em caso de token
// ausente ou inválido.
func Auth(issuer port.TokenIssuer) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		if header == "" {
			return unauthorized(c)
		}

		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			return unauthorized(c)
		}

		token := strings.TrimSpace(header[len(prefix):])
		if token == "" {
			return unauthorized(c)
		}

		userID, err := issuer.Parse(token)
		if err != nil || userID == "" {
			return unauthorized(c)
		}

		c.Locals(LocalsUserID, userID)
		return c.Next()
	}
}

func unauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
}
