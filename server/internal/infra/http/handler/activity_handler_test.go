package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
)

func TestActivityHandler_RegisterActivity_Success(t *testing.T) {
	mocks := newHandlerMocks()
	h := handler.NewActivityHandler(mocks.registerActivity)

	body := `{
		"user_id": "user-1",
		"activity_type": "running",
		"date": "2025-06-01T08:00:00Z",
		"duration_minutes": 30,
		"avg_heart_rate": 150
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["id"] == "" {
		t.Fatal("expected non-empty ID in response")
	}
}

func TestActivityHandler_RegisterActivity_InvalidBody(t *testing.T) {
	mocks := newHandlerMocks()
	h := handler.NewActivityHandler(mocks.registerActivity)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestActivityHandler_RegisterActivity_ValidationError(t *testing.T) {
	mocks := newHandlerMocks()
	h := handler.NewActivityHandler(mocks.registerActivity)

	body := `{
		"user_id": "user-1",
		"activity_type": "flying",
		"date": "2025-06-01T08:00:00Z",
		"duration_minutes": 30,
		"avg_heart_rate": 150
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}
