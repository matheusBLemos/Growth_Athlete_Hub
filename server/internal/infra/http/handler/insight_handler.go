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

type generateInsightsRequest struct {
	UserID string `json:"user_id"`
}

func (h *InsightHandler) Generate(c *fiber.Ctx) error {
	var req generateInsightsRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid request body")
	}

	output, err := h.generateInsights.Execute(c.Context(), usecase.GenerateInsightsInput{
		UserID: req.UserID,
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
