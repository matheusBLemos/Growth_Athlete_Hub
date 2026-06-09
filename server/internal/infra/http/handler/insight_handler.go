package handler

import (
	"encoding/json"
	"errors"
	"net/http"

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

func (h *InsightHandler) Generate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req generateInsightsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	output, err := h.generateInsights.Execute(r.Context(), usecase.GenerateInsightsInput{
		UserID: req.UserID,
	})
	if err != nil {
		if errors.Is(err, entity.ErrEmptyUserID) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":    output.Count,
		"insights": output.Insights,
	})
}
