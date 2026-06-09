package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type MetricHandler struct {
	recordMetric *usecase.RecordMetric
	queryMetrics *usecase.QueryMetrics
}

func NewMetricHandler(record *usecase.RecordMetric, query *usecase.QueryMetrics) *MetricHandler {
	return &MetricHandler{
		recordMetric: record,
		queryMetrics: query,
	}
}

type recordMetricRequest struct {
	MetricType string  `json:"metric_type"`
	Value      float64 `json:"value"`
	Date       string  `json:"date"`
}

func (h *MetricHandler) Record(c *fiber.Ctx) error {
	userID := userIDFromCtx(c)

	var req recordMetricRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid request body")
	}

	date, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid date format, use RFC3339")
	}

	input := usecase.RecordMetricInput{
		UserID:     userID,
		MetricType: req.MetricType,
		Value:      req.Value,
		Date:       date,
	}

	output, err := h.recordMetric.Execute(c.Context(), input)
	if err != nil {
		if errors.Is(err, valueobject.ErrInvalidMetricType) ||
			errors.Is(err, entity.ErrMetricOutOfRange) ||
			errors.Is(err, entity.ErrEmptyUserID) ||
			errors.Is(err, entity.ErrEmptyDate) {
			return writeError(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		return writeError(c, fiber.StatusInternalServerError, "internal error")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": output.ID})
}

func (h *MetricHandler) Query(c *fiber.Ctx) error {
	userID := userIDFromCtx(c)
	metricType := c.Query("metric_type")
	fromStr := c.Query("from")
	toStr := c.Query("to")

	if metricType == "" || fromStr == "" || toStr == "" {
		return writeError(c, fiber.StatusBadRequest, "missing required query params: metric_type, from, to")
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid 'from' date format")
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid 'to' date format")
	}

	input := usecase.QueryMetricsInput{
		UserID:     userID,
		MetricType: metricType,
		From:       from,
		To:         to,
	}

	output, err := h.queryMetrics.Execute(c.Context(), input)
	if err != nil {
		if errors.Is(err, valueobject.ErrInvalidMetricType) {
			return writeError(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		return writeError(c, fiber.StatusInternalServerError, "internal error")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"metrics": output.Metrics,
		"count":   len(output.Metrics),
	})
}
