package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

type InsightHandler struct {
	generateInsights *usecase.GenerateInsights
}

func NewInsightHandler(generate *usecase.GenerateInsights) *InsightHandler {
	return &InsightHandler{generateInsights: generate}
}

func (h *InsightHandler) Generate(c *fiber.Ctx) error {
	userID := userIDFromCtx(c)

	output, err := h.generateInsights.Execute(c.UserContext(), usecase.GenerateInsightsInput{
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, entity.ErrEmptyUserID) {
			return writeError(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		return writeError(c, fiber.StatusInternalServerError, "internal error")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"count":    output.Count,
		"insights": output.Insights,
	})
}
