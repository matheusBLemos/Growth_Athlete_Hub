package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
)

type fakeDeviceRepo struct {
	saved []port.Device
	err   error
}

func (r *fakeDeviceRepo) Save(_ context.Context, userID, token, platform string) error {
	if r.err != nil {
		return r.err
	}
	r.saved = append(r.saved, port.Device{UserID: userID, Token: token, Platform: platform})
	return nil
}

func (r *fakeDeviceRepo) FindByUser(_ context.Context, userID string) ([]port.Device, error) {
	if r.err != nil {
		return nil, r.err
	}
	var out []port.Device
	for _, d := range r.saved {
		if d.UserID == userID {
			out = append(out, d)
		}
	}
	return out, nil
}

var _ port.DeviceRepository = (*fakeDeviceRepo)(nil)

func TestDeviceHandler_Register_Success(t *testing.T) {
	repo := &fakeDeviceRepo{}
	app := fiber.New()
	app.Post("/api/v1/notifications/devices", withUser("user-1"), handler.NewDeviceHandler(repo).Register)

	body := `{"token":"device-token-abc","platform":"android"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/devices", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("saved %d devices, want 1", len(repo.saved))
	}
	d := repo.saved[0]
	if d.UserID != "user-1" || d.Token != "device-token-abc" || d.Platform != "android" {
		t.Errorf("saved = %+v", d)
	}
}

func TestDeviceHandler_Register_InvalidBody(t *testing.T) {
	repo := &fakeDeviceRepo{}
	app := fiber.New()
	app.Post("/api/v1/notifications/devices", withUser("user-1"), handler.NewDeviceHandler(repo).Register)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/devices", strings.NewReader(`{invalid`))
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

func TestDeviceHandler_Register_EmptyToken(t *testing.T) {
	repo := &fakeDeviceRepo{}
	app := fiber.New()
	app.Post("/api/v1/notifications/devices", withUser("user-1"), handler.NewDeviceHandler(repo).Register)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/devices", strings.NewReader(`{"token":"  ","platform":"ios"}`))
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

func TestDeviceHandler_Register_RepoError(t *testing.T) {
	repo := &fakeDeviceRepo{err: errors.New("db down")}
	app := fiber.New()
	app.Post("/api/v1/notifications/devices", withUser("user-1"), handler.NewDeviceHandler(repo).Register)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/devices", strings.NewReader(`{"token":"t","platform":"web"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}
