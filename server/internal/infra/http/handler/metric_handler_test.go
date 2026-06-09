package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
)

func TestMetricHandler_RecordMetric_Success(t *testing.T) {
	mocks := newHandlerMocks()
	h := handler.NewMetricHandler(mocks.recordMetric, mocks.queryMetrics)

	body := `{
		"user_id": "user-1",
		"metric_type": "hrv",
		"value": 65.0,
		"date": "2025-06-01T00:00:00Z"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Record(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["id"] == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestMetricHandler_QueryMetrics_Success(t *testing.T) {
	mocks := newHandlerMocks()
	h := handler.NewMetricHandler(mocks.recordMetric, mocks.queryMetrics)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics?user_id=user-1&metric_type=hrv&from=2025-06-01T00:00:00Z&to=2025-06-30T00:00:00Z", nil)
	w := httptest.NewRecorder()

	h.Query(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMetricHandler_QueryMetrics_MissingParams(t *testing.T) {
	mocks := newHandlerMocks()
	h := handler.NewMetricHandler(mocks.recordMetric, mocks.queryMetrics)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	w := httptest.NewRecorder()

	h.Query(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
