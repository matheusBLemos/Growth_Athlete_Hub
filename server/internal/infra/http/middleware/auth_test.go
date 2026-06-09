package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/middleware"
)

type fakeIssuer struct{}

func (fakeIssuer) Issue(userID string) (string, error) { return "token:" + userID, nil }

func (fakeIssuer) Parse(token string) (string, error) {
	const prefix = "token:"
	if len(token) <= len(prefix) || token[:len(prefix)] != prefix {
		return "", errors.New("invalid token")
	}
	return token[len(prefix):], nil
}

func newApp() *fiber.App {
	app := fiber.New()
	app.Use(middleware.Auth(fakeIssuer{}))
	app.Get("/protected", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"userID": c.Locals("userID")})
	})
	return app
}

func TestAuth_ValidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer token:user-1")

	resp, err := newApp().Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)

	resp, err := newApp().Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_MalformedHeader(t *testing.T) {
	cases := []string{"token:user-1", "Bearer", "Basic abc", "Bearer "}
	for _, h := range cases {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", h)

		resp, err := newApp().Test(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("header %q: expected 401, got %d", h, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer garbage")

	resp, err := newApp().Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
