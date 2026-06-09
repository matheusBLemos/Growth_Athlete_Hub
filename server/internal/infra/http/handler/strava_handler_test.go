package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
)

// --- fakes específicos do conector Strava (gateway + token repo) ---

type fakeStravaGateway struct {
	exchanged  port.ProviderToken
	activities []port.ProviderActivity
}

func (g *fakeStravaGateway) AuthURL(state string) string {
	return "https://strava.test/oauth/authorize?state=" + url.QueryEscape(state)
}
func (g *fakeStravaGateway) ExchangeCode(_ context.Context, _ string) (port.ProviderToken, error) {
	t := g.exchanged
	if t.Provider == "" {
		t.Provider = "strava"
	}
	if t.AccessToken == "" {
		t.AccessToken = "acc"
	}
	t.ExpiresAt = time.Now().Add(time.Hour)
	return t, nil
}
func (g *fakeStravaGateway) Refresh(_ context.Context, _ string) (port.ProviderToken, error) {
	return port.ProviderToken{Provider: "strava", AccessToken: "acc", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (g *fakeStravaGateway) FetchActivities(_ context.Context, _ string, _ time.Time) ([]port.ProviderActivity, error) {
	return g.activities, nil
}
func (g *fakeStravaGateway) Provider() string { return "strava" }

var _ port.ProviderGateway = (*fakeStravaGateway)(nil)

type fakeTokenRepo struct {
	tokens map[string]*port.ProviderToken
}

func newFakeTokenRepo() *fakeTokenRepo { return &fakeTokenRepo{tokens: make(map[string]*port.ProviderToken)} }

func (r *fakeTokenRepo) Save(_ context.Context, userID string, token port.ProviderToken) error {
	t := token
	r.tokens[userID+":"+token.Provider] = &t
	return nil
}
func (r *fakeTokenRepo) Find(_ context.Context, userID, provider string) (*port.ProviderToken, error) {
	return r.tokens[userID+":"+provider], nil
}
func (r *fakeTokenRepo) FindUserByAthlete(_ context.Context, provider, athleteID string) (string, bool, error) {
	for key, tok := range r.tokens {
		if tok.Provider == provider && tok.AthleteID == athleteID {
			return key[:len(key)-len(provider)-1], true, nil
		}
	}
	return "", false, nil
}

var _ port.ProviderTokenRepository = (*fakeTokenRepo)(nil)

func newStravaHandler(gw port.ProviderGateway, repo port.ProviderTokenRepository) *handler.StravaHandler {
	connect := usecase.NewConnectProvider(gw, repo)
	sync := usecase.NewSyncProviderActivities(gw, repo, &noopPublisher{})
	return handler.NewStravaHandler(connect, sync, &noopPublisher{}, fakeIssuer{}, "verify-token")
}

func TestStravaHandler_Connect_Redirects(t *testing.T) {
	h := newStravaHandler(&fakeStravaGateway{}, newFakeTokenRepo())
	app := fiber.New()
	app.Get("/connect", withUser("user-1"), h.Connect)

	req := httptest.NewRequest(http.MethodGet, "/connect", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "https://strava.test/oauth/authorize?state=") {
		t.Fatalf("location = %q", loc)
	}
	// O state deve carregar o userID assinado (fakeIssuer => "token:user-1").
	if !strings.Contains(loc, url.QueryEscape("token:user-1")) {
		t.Fatalf("state should embed signed userID, got location %q", loc)
	}
}

func TestStravaHandler_Callback_PersistsToken(t *testing.T) {
	repo := newFakeTokenRepo()
	h := newStravaHandler(&fakeStravaGateway{}, repo)
	app := fiber.New()
	app.Get("/callback", h.Callback)

	// state válido = token assinado do fakeIssuer
	req := httptest.NewRequest(http.MethodGet, "/callback?code=the-code&state=token:user-1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if repo.tokens["user-1:strava"] == nil {
		t.Fatal("token not persisted for user-1")
	}
}

func TestStravaHandler_Callback_InvalidState(t *testing.T) {
	h := newStravaHandler(&fakeStravaGateway{}, newFakeTokenRepo())
	app := fiber.New()
	app.Get("/callback", h.Callback)

	req := httptest.NewRequest(http.MethodGet, "/callback?code=the-code&state=garbage", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestStravaHandler_Callback_MissingCode(t *testing.T) {
	h := newStravaHandler(&fakeStravaGateway{}, newFakeTokenRepo())
	app := fiber.New()
	app.Get("/callback", h.Callback)

	req := httptest.NewRequest(http.MethodGet, "/callback?state=token:user-1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestStravaHandler_Sync_Success(t *testing.T) {
	repo := newFakeTokenRepo()
	repo.tokens["user-1:strava"] = &port.ProviderToken{Provider: "strava", AccessToken: "acc", ExpiresAt: time.Now().Add(time.Hour)}
	gw := &fakeStravaGateway{activities: []port.ProviderActivity{
		{Provider: "strava", ExternalID: "1", Type: "running", StartTime: time.Now().Add(-time.Hour), Duration: time.Minute},
	}}
	h := newStravaHandler(gw, repo)
	app := fiber.New()
	app.Post("/sync", withUser("user-1"), h.Sync)

	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestStravaHandler_Sync_NotConnected(t *testing.T) {
	h := newStravaHandler(&fakeStravaGateway{}, newFakeTokenRepo())
	app := fiber.New()
	app.Post("/sync", withUser("user-1"), h.Sync)

	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestStravaHandler_WebhookVerify(t *testing.T) {
	h := newStravaHandler(&fakeStravaGateway{}, newFakeTokenRepo())
	app := fiber.New()
	app.Get("/webhook", h.WebhookVerify)

	req := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=verify-token&hub.challenge=abc123", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["hub.challenge"] != "abc123" {
		t.Fatalf("hub.challenge = %q, want abc123", body["hub.challenge"])
	}
}

func TestStravaHandler_WebhookVerify_BadToken(t *testing.T) {
	h := newStravaHandler(&fakeStravaGateway{}, newFakeTokenRepo())
	app := fiber.New()
	app.Get("/webhook", h.WebhookVerify)

	req := httptest.NewRequest(http.MethodGet, "/webhook?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=abc123", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestStravaHandler_WebhookEvent(t *testing.T) {
	pub := &recordingPublisher{}
	connect := usecase.NewConnectProvider(&fakeStravaGateway{}, newFakeTokenRepo())
	sync := usecase.NewSyncProviderActivities(&fakeStravaGateway{}, newFakeTokenRepo(), &noopPublisher{})
	h := handler.NewStravaHandler(connect, sync, pub, fakeIssuer{}, "verify-token")

	app := fiber.New()
	app.Post("/webhook", h.WebhookEvent)

	body := `{"object_type":"activity","aspect_type":"create","object_id":999,"owner_id":424242}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(pub.events) != 1 {
		t.Fatalf("events published = %d, want 1", len(pub.events))
	}
	if pub.events[0].Type != "strava.webhook.activity" {
		t.Fatalf("event type = %q", pub.events[0].Type)
	}
}

// recordingPublisher captura eventos publicados (handler usa noopPublisher por padrão).
type recordingPublisher struct {
	events []port.Event
}

func (p *recordingPublisher) Publish(_ context.Context, e port.Event) error {
	p.events = append(p.events, e)
	return nil
}

var _ port.EventPublisher = (*recordingPublisher)(nil)
