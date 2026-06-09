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

func newAuthApp(mocks handlerMocks) *fiber.App {
	app := fiber.New()
	h := handler.NewAuthHandler(mocks.registerUser, mocks.loginUser)
	app.Post("/api/v1/auth/register", h.Register)
	app.Post("/api/v1/auth/login", h.Login)
	return app
}

func TestAuthHandler_Register_Success(t *testing.T) {
	app := newAuthApp(newHandlerMocks())

	body := `{
		"name": "Matheus",
		"email": "matheus@email.com",
		"password": "s3cret-pass",
		"birth_date": "2000-01-01T00:00:00Z"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
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
		t.Fatalf("decode: %v", err)
	}
	if out["id"] == "" {
		t.Fatal("expected non-empty id")
	}
}

func TestAuthHandler_Register_InvalidBody(t *testing.T) {
	app := newAuthApp(newHandlerMocks())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{invalid`))
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

func TestAuthHandler_Register_InvalidEmail(t *testing.T) {
	app := newAuthApp(newHandlerMocks())
	body := `{"name":"M","email":"bad","password":"s3cret-pass","birth_date":"2000-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
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

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	app := newAuthApp(newHandlerMocks())
	body := `{"name":"M","email":"dup@email.com","password":"s3cret-pass","birth_date":"2000-01-01T00:00:00Z"}`

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if i == 1 && resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409 on duplicate, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	app := newAuthApp(newHandlerMocks())

	reg := `{"name":"M","email":"login@email.com","password":"s3cret-pass","birth_date":"2000-01-01T00:00:00Z"}`
	rr := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(reg))
	rr.Header.Set("Content-Type", "application/json")
	if _, err := app.Test(rr); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	login := `{"email":"login@email.com","password":"s3cret-pass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(login))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["token"] == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestAuthHandler_Login_BadCredentials(t *testing.T) {
	app := newAuthApp(newHandlerMocks())
	login := `{"email":"ghost@email.com","password":"nope"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(login))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
