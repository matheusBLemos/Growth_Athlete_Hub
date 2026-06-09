package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type ActivityHandler struct {
	registerActivity *usecase.RegisterActivity
}

func NewActivityHandler(register *usecase.RegisterActivity) *ActivityHandler {
	return &ActivityHandler{registerActivity: register}
}

type registerActivityRequest struct {
	ActivityType    string  `json:"activity_type"`
	Date            string  `json:"date"`
	DurationMinutes float64 `json:"duration_minutes"`
	AvgHeartRate    int     `json:"avg_heart_rate"`
	ExternalID      string  `json:"external_id,omitempty"`
}

func (h *ActivityHandler) Register(c *fiber.Ctx) error {
	userID := userIDFromCtx(c)

	var req registerActivityRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid request body")
	}

	date, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid date format, use RFC3339")
	}

	input := usecase.RegisterActivityInput{
		UserID:       userID,
		ActivityType: req.ActivityType,
		Date:         date,
		Duration:     time.Duration(req.DurationMinutes * float64(time.Minute)),
		AvgHeartRate: req.AvgHeartRate,
		ExternalID:   req.ExternalID,
	}

	output, err := h.registerActivity.Execute(c.Context(), input)
	if err != nil {
		if errors.Is(err, valueobject.ErrInvalidActivityType) ||
			errors.Is(err, entity.ErrInvalidDuration) ||
			errors.Is(err, entity.ErrHROutOfRange) ||
			errors.Is(err, entity.ErrActivityDateFuture) ||
			errors.Is(err, entity.ErrEmptyUserID) {
			return writeError(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		if errors.Is(err, usecase.ErrDuplicateActivity) {
			return writeError(c, fiber.StatusConflict, err.Error())
		}
		return writeError(c, fiber.StatusInternalServerError, "internal error")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": output.ID})
}
