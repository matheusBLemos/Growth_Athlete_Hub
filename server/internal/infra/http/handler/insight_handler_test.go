package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
)

func TestInsightHandler_Generate_Success(t *testing.T) {
	mocks := newHandlerMocks()
	h := handler.NewInsightHandler(mocks.generateInsights)

	body := `{"user_id": "user-1"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/insights/generate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Generate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["count"] == nil {
		t.Fatal("expected count in response")
	}
}

func TestInsightHandler_Generate_EmptyUserID(t *testing.T) {
	mocks := newHandlerMocks()
	h := handler.NewInsightHandler(mocks.generateInsights)

	body := `{"user_id": ""}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/insights/generate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Generate(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}
