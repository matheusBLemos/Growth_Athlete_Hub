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

type ActivityHandler struct {
	registerActivity *usecase.RegisterActivity
}

func NewActivityHandler(register *usecase.RegisterActivity) *ActivityHandler {
	return &ActivityHandler{registerActivity: register}
}

type registerActivityRequest struct {
	UserID          string  `json:"user_id"`
	ActivityType    string  `json:"activity_type"`
	Date            string  `json:"date"`
	DurationMinutes float64 `json:"duration_minutes"`
	AvgHeartRate    int     `json:"avg_heart_rate"`
	ExternalID      string  `json:"external_id,omitempty"`
}

func (h *ActivityHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerActivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	date, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date format, use RFC3339")
		return
	}

	input := usecase.RegisterActivityInput{
		UserID:       req.UserID,
		ActivityType: req.ActivityType,
		Date:         date,
		Duration:     time.Duration(req.DurationMinutes * float64(time.Minute)),
		AvgHeartRate: req.AvgHeartRate,
		ExternalID:   req.ExternalID,
	}

	output, err := h.registerActivity.Execute(r.Context(), input)
	if err != nil {
		if errors.Is(err, valueobject.ErrInvalidActivityType) ||
			errors.Is(err, entity.ErrInvalidDuration) ||
			errors.Is(err, entity.ErrHROutOfRange) ||
			errors.Is(err, entity.ErrActivityDateFuture) ||
			errors.Is(err, entity.ErrEmptyUserID) {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrDuplicateActivity) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": output.ID})
}
