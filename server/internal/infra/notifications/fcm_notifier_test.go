package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

func TestFCMNotifier_Send_RequestShape(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotAuth   string
		gotCType  string
		gotBody   fcmRequest
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":1}`))
	}))
	defer srv.Close()

	n := NewFCMNotifier(FCMConfig{BaseURL: srv.URL, ServerKey: "secret-key"})

	err := n.Send(context.Background(), port.Notification{
		UserID: "u1",
		Token:  "device-token-123",
		Title:  "Recovery: low",
		Body:   "Take it easy today.",
		Data:   map[string]string{"insight_id": "ins-9"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/" {
		t.Errorf("path = %s, want /", gotPath)
	}
	if gotAuth != "key=secret-key" {
		t.Errorf("authorization = %q, want key=secret-key", gotAuth)
	}
	if gotCType != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotCType)
	}
	if gotBody.To != "device-token-123" {
		t.Errorf("to = %q, want device-token-123", gotBody.To)
	}
	if gotBody.Notification.Title != "Recovery: low" {
		t.Errorf("notification.title = %q", gotBody.Notification.Title)
	}
	if gotBody.Notification.Body != "Take it easy today." {
		t.Errorf("notification.body = %q", gotBody.Notification.Body)
	}
	if gotBody.Data["insight_id"] != "ins-9" {
		t.Errorf("data[insight_id] = %q, want ins-9", gotBody.Data["insight_id"])
	}
}

func TestFCMNotifier_Send_Non2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	n := NewFCMNotifier(FCMConfig{BaseURL: srv.URL, ServerKey: "bad-key"})

	err := n.Send(context.Background(), port.Notification{Token: "t", Title: "x", Body: "y"})
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}

func TestNewFCMNotifier_DefaultsBaseURL(t *testing.T) {
	n := NewFCMNotifier(FCMConfig{ServerKey: "k"})
	if n.cfg.BaseURL != defaultFCMBaseURL {
		t.Errorf("base url = %q, want default %q", n.cfg.BaseURL, defaultFCMBaseURL)
	}
}
