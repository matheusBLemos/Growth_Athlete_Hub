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

func TestActivityHandler_RegisterActivity_Success(t *testing.T) {
	mocks := newHandlerMocks()
	app := fiber.New()
	app.Post("/api/v1/activities", handler.NewActivityHandler(mocks.registerActivity).Register)

	body := `{
		"user_id": "user-1",
		"activity_type": "running",
		"date": "2025-06-01T08:00:00Z",
		"duration_minutes": 30,
		"avg_heart_rate": 150
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities", strings.NewReader(body))
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
		t.Fatal("expected non-empty ID in response")
	}
}

func TestActivityHandler_RegisterActivity_InvalidBody(t *testing.T) {
	mocks := newHandlerMocks()
	app := fiber.New()
	app.Post("/api/v1/activities", handler.NewActivityHandler(mocks.registerActivity).Register)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities", strings.NewReader(`{invalid`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestActivityHandler_RegisterActivity_ValidationError(t *testing.T) {
	mocks := newHandlerMocks()
	app := fiber.New()
	app.Post("/api/v1/activities", handler.NewActivityHandler(mocks.registerActivity).Register)

	body := `{
		"user_id": "user-1",
		"activity_type": "flying",
		"date": "2025-06-01T08:00:00Z",
		"duration_minutes": 30,
		"avg_heart_rate": 150
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities", strings.NewReader(body))
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
