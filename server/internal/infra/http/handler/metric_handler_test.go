package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
)

func newMetricApp(mocks handlerMocks) *fiber.App {
	app := fiber.New()
	h := handler.NewMetricHandler(mocks.recordMetric, mocks.queryMetrics)
	app.Post("/api/v1/metrics", withUser("user-1"), h.Record)
	app.Get("/api/v1/metrics", withUser("user-1"), h.Query)
	return app
}

func TestMetricHandler_RecordMetric_Success(t *testing.T) {
	app := newMetricApp(newHandlerMocks())

	body := `{
		"user_id": "user-1",
		"metric_type": "hrv",
		"value": 65.0,
		"date": "2025-06-01T00:00:00Z"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/metrics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var out map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if out["id"] == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestMetricHandler_QueryMetrics_Success(t *testing.T) {
	app := newMetricApp(newHandlerMocks())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics?user_id=user-1&metric_type=hrv&from=2025-06-01T00:00:00Z&to=2025-06-30T00:00:00Z", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMetricHandler_QueryMetrics_MissingParams(t *testing.T) {
	app := newMetricApp(newHandlerMocks())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
