package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
)

type fakeNotificationRepo struct {
	byUser map[string][]port.NotificationRecord
	err    error
	gotLim int
}

func (r *fakeNotificationRepo) Save(_ context.Context, record port.NotificationRecord) error {
	if r.err != nil {
		return r.err
	}
	if r.byUser == nil {
		r.byUser = make(map[string][]port.NotificationRecord)
	}
	r.byUser[record.UserID] = append(r.byUser[record.UserID], record)
	return nil
}

func (r *fakeNotificationRepo) ListByUser(_ context.Context, userID string, limit int) ([]port.NotificationRecord, error) {
	r.gotLim = limit
	if r.err != nil {
		return nil, r.err
	}
	return r.byUser[userID], nil
}

var _ port.NotificationRepository = (*fakeNotificationRepo)(nil)

func TestNotificationHandler_List_Success(t *testing.T) {
	repo := &fakeNotificationRepo{byUser: map[string][]port.NotificationRecord{
		"user-1": {
			{ID: "n1", UserID: "user-1", InsightID: "ins-1", Type: "recovery", Severity: "warning", Title: "T", Body: "B", Status: port.NotificationStatusSent, CreatedAt: time.Now()},
			{ID: "n2", UserID: "user-1", InsightID: "ins-2", Status: port.NotificationStatusFailed, Error: "boom", CreatedAt: time.Now()},
		},
	}}
	app := fiber.New()
	app.Get("/api/v1/notifications", withUser("user-1"), handler.NewNotificationHandler(repo).List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Notifications []map[string]any `json:"notifications"`
		Count         int              `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Count != 2 || len(body.Notifications) != 2 {
		t.Fatalf("count = %d, notifications = %d, want 2 and 2", body.Count, len(body.Notifications))
	}
	if body.Notifications[0]["status"] != "sent" {
		t.Errorf("first status = %v, want sent", body.Notifications[0]["status"])
	}
	if body.Notifications[1]["error"] != "boom" {
		t.Errorf("second error = %v, want boom", body.Notifications[1]["error"])
	}
}

func TestNotificationHandler_List_DefaultLimit(t *testing.T) {
	repo := &fakeNotificationRepo{}
	app := fiber.New()
	app.Get("/api/v1/notifications", withUser("user-1"), handler.NewNotificationHandler(repo).List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if repo.gotLim != 50 {
		t.Errorf("default limit = %d, want 50", repo.gotLim)
	}
}

func TestNotificationHandler_List_InvalidLimit(t *testing.T) {
	repo := &fakeNotificationRepo{}
	app := fiber.New()
	app.Get("/api/v1/notifications", withUser("user-1"), handler.NewNotificationHandler(repo).List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?limit=abc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestNotificationHandler_List_RepoError(t *testing.T) {
	repo := &fakeNotificationRepo{err: errors.New("db down")}
	app := fiber.New()
	app.Get("/api/v1/notifications", withUser("user-1"), handler.NewNotificationHandler(repo).List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}
