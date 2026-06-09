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

func TestInsightHandler_Generate_Success(t *testing.T) {
	mocks := newHandlerMocks()
	app := fiber.New()
	app.Post("/api/v1/insights/generate", handler.NewInsightHandler(mocks.generateInsights).Generate)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/insights/generate", strings.NewReader(`{"user_id": "user-1"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if out["count"] == nil {
		t.Fatal("expected count in response")
	}
}

func TestInsightHandler_Generate_EmptyUserID(t *testing.T) {
	mocks := newHandlerMocks()
	app := fiber.New()
	app.Post("/api/v1/insights/generate", handler.NewInsightHandler(mocks.generateInsights).Generate)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/insights/generate", strings.NewReader(`{"user_id": ""}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}
