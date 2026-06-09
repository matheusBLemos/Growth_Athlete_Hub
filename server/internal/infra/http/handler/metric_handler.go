package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

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
	UserID     string  `json:"user_id"`
	MetricType string  `json:"metric_type"`
	Value      float64 `json:"value"`
	Date       string  `json:"date"`
}

func (h *MetricHandler) Record(w http.ResponseWriter, r *http.Request) {
	var req recordMetricRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	date, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format, use RFC3339")
		return
	}

	input := usecase.RecordMetricInput{
		UserID:     req.UserID,
		MetricType: req.MetricType,
		Value:      req.Value,
		Date:       date,
	}

	output, err := h.recordMetric.Execute(r.Context(), input)
	if err != nil {
		if errors.Is(err, valueobject.ErrInvalidMetricType) ||
			errors.Is(err, entity.ErrMetricOutOfRange) ||
			errors.Is(err, entity.ErrEmptyUserID) ||
			errors.Is(err, entity.ErrEmptyDate) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": output.ID})
}

func (h *MetricHandler) Query(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	metricType := r.URL.Query().Get("metric_type")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if userID == "" || metricType == "" || fromStr == "" || toStr == "" {
		writeError(w, http.StatusBadRequest, "missing required query params: user_id, metric_type, from, to")
		return
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid 'from' date format")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid 'to' date format")
		return
	}

	input := usecase.QueryMetricsInput{
		UserID:     userID,
		MetricType: metricType,
		From:       from,
		To:         to,
	}

	output, err := h.queryMetrics.Execute(r.Context(), input)
	if err != nil {
		if errors.Is(err, valueobject.ErrInvalidMetricType) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"metrics": output.Metrics,
		"count":   len(output.Metrics),
	})
}
