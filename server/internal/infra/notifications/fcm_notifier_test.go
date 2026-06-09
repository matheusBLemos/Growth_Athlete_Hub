package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// staticToken devolve um TokenSource com um bearer token fixo, sem rede.
func staticToken(tok string) oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
}

func TestFCMNotifier_Send_RequestShape(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotAuth   string
		gotCType  string
		gotBody   fcmV1Request
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"projects/proj-1/messages/123"}`))
	}))
	defer srv.Close()

	n := NewFCMNotifier(FCMConfig{
		BaseURL:     srv.URL,
		ProjectID:   "proj-1",
		TokenSource: staticToken("test-bearer-token"),
	})

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
	if gotPath != "/v1/projects/proj-1/messages:send" {
		t.Errorf("path = %s, want /v1/projects/proj-1/messages:send", gotPath)
	}
	if gotAuth != "Bearer test-bearer-token" {
		t.Errorf("authorization = %q, want Bearer test-bearer-token", gotAuth)
	}
	if gotCType != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotCType)
	}
	if gotBody.Message.Token != "device-token-123" {
		t.Errorf("message.token = %q, want device-token-123", gotBody.Message.Token)
	}
	if gotBody.Message.Notification.Title != "Recovery: low" {
		t.Errorf("message.notification.title = %q", gotBody.Message.Notification.Title)
	}
	if gotBody.Message.Notification.Body != "Take it easy today." {
		t.Errorf("message.notification.body = %q", gotBody.Message.Notification.Body)
	}
	if gotBody.Message.Data["insight_id"] != "ins-9" {
		t.Errorf("message.data[insight_id] = %q, want ins-9", gotBody.Message.Data["insight_id"])
	}
}

func TestFCMNotifier_Send_Non2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"status":"UNAUTHENTICATED"}}`))
	}))
	defer srv.Close()

	n := NewFCMNotifier(FCMConfig{
		BaseURL:     srv.URL,
		ProjectID:   "proj-1",
		TokenSource: staticToken("bad-token"),
	})

	err := n.Send(context.Background(), port.Notification{Token: "t", Title: "x", Body: "y"})
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}

func TestFCMNotifier_Send_NilTokenSource_ReturnsError(t *testing.T) {
	n := NewFCMNotifier(FCMConfig{BaseURL: "https://example.test", ProjectID: "proj-1"})
	err := n.Send(context.Background(), port.Notification{Token: "t", Title: "x", Body: "y"})
	if err == nil {
		t.Fatal("expected error when token source is nil")
	}
}

func TestNewFCMNotifier_DefaultsBaseURL(t *testing.T) {
	n := NewFCMNotifier(FCMConfig{ProjectID: "proj-1", TokenSource: staticToken("k")})
	if n.cfg.BaseURL != defaultFCMBaseURL {
		t.Errorf("base url = %q, want default %q", n.cfg.BaseURL, defaultFCMBaseURL)
	}
}
