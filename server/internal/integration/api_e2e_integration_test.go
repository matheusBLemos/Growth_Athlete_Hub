//go:build integration

// Teste de integração ponta-a-ponta da API HTTP. Monta o router REAL (Fiber)
// com repositórios Postgres reais, hasher Argon2id real, JWT issuer real e um
// publisher/cache que usam RabbitMQ/Redis reais quando TEST_RABBITMQ_URL /
// TEST_REDIS_ADDR estão setados, caindo em fakes in-process (implementando as
// portas) caso contrário. Gated em TEST_DATABASE_URL.
//
// A jornada exercitada: register -> login (JWT) -> POST activity -> POST metrics
// -> POST /insights/generate -> register device -> GET /notifications, além de
// verificar 401 sem o token.
package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/auth"
	rediscache "github.com/Growth-Athlete-Hub/gah-server/internal/infra/cache/redis"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/connectors/strava"
	router "github.com/Growth-Athlete-Hub/gah-server/internal/infra/http"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/insights/deterministic"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/messaging/rabbitmq"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"
)

// fakePublisher implementa port.EventPublisher in-process (sem broker).
type fakePublisher struct {
	mu     sync.Mutex
	events []port.Event
}

func (p *fakePublisher) Publish(_ context.Context, e port.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, e)
	return nil
}

// fakeCache implementa port.Cache em memória.
type fakeCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func newFakeCache() *fakeCache { return &fakeCache{m: make(map[string][]byte)} }

func (c *fakeCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[key]
	return v, ok, nil
}

func (c *fakeCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = value
	return nil
}

func (c *fakeCache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		delete(c.m, k)
	}
	return nil
}

// buildPublisher escolhe o publisher real (se TEST_RABBITMQ_URL) ou um fake.
func buildPublisher(t *testing.T) port.EventPublisher {
	t.Helper()
	if url := os.Getenv("TEST_RABBITMQ_URL"); url != "" {
		pub, err := rabbitmq.NewPublisher(url, "gahtest.e2e."+time.Now().Format("150405.000000"))
		if err != nil {
			t.Fatalf("new real publisher: %v", err)
		}
		t.Cleanup(func() { _ = pub.Close() })
		return pub
	}
	return &fakePublisher{}
}

// buildCache escolhe o cache real (se TEST_REDIS_ADDR) ou um fake.
func buildCache(t *testing.T) port.Cache {
	t.Helper()
	if addr := os.Getenv("TEST_REDIS_ADDR"); addr != "" {
		c, err := rediscache.New(rediscache.Config{Addr: addr})
		if err != nil {
			t.Fatalf("new real cache: %v", err)
		}
		if err := c.Ping(context.Background()); err != nil {
			t.Fatalf("ping real cache: %v", err)
		}
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
	return newFakeCache()
}

// buildApp monta o router real com toda a wiring. Pula se TEST_DATABASE_URL
// não estiver setado.
func buildApp(t *testing.T) *fiber.App {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping HTTP E2E integration test")
	}

	db, err := postgres.NewDB(dsn)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := postgres.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	activityRepo := postgres.NewActivityRepository(db)
	metricRepo := postgres.NewMetricRepository(db)
	insightRepo := postgres.NewInsightRepository(db)
	userRepo := postgres.NewUserRepository(db)
	providerTokenRepo := postgres.NewProviderTokenRepository(db)
	deviceRepo := postgres.NewDeviceRepository(db)
	notificationRepo := postgres.NewNotificationRepository(db)

	hasher := auth.NewArgon2Hasher("test-pepper")
	tokenIssuer := auth.NewJWTIssuer("test-secret", time.Hour)

	evaluator := deterministic.NewCompositeEvaluator(
		deterministic.NewHRVRule(),
		deterministic.NewRestingHRRule(),
		deterministic.NewSleepRule(),
		deterministic.NewACWRRule(),
		deterministic.NewRecoveryRule(),
	)

	publisher := buildPublisher(t)
	cache := buildCache(t)
	ttl := 5 * time.Minute

	registerActivity := usecase.NewRegisterActivity(activityRepo, publisher)
	recordMetric := usecase.NewRecordMetric(metricRepo, publisher, cache, ttl)
	queryMetrics := usecase.NewQueryMetrics(metricRepo, cache, ttl)
	generateInsights := usecase.NewGenerateInsights(metricRepo, insightRepo, evaluator)
	registerUser := usecase.NewRegisterUser(userRepo, hasher, publisher)
	loginUser := usecase.NewLoginUser(userRepo, hasher, tokenIssuer)
	// Gateway Strava real, mas só usamos AuthURL (puro string-building, sem rede).
	stravaGateway := strava.NewGateway(strava.Config{
		ClientID:    "test-client",
		RedirectURL: "http://localhost:8080/api/v1/connectors/strava/callback",
	})
	connectProvider := usecase.NewConnectProvider(stravaGateway, providerTokenRepo)
	syncProvider := usecase.NewSyncProviderActivities(stravaGateway, providerTokenRepo, publisher)

	authHandler := handler.NewAuthHandler(registerUser, loginUser)
	activityHandler := handler.NewActivityHandler(registerActivity)
	metricHandler := handler.NewMetricHandler(recordMetric, queryMetrics)
	insightHandler := handler.NewInsightHandler(generateInsights)
	stravaHandler := handler.NewStravaHandler(connectProvider, syncProvider, publisher, tokenIssuer, "webhook-token")
	deviceHandler := handler.NewDeviceHandler(deviceRepo)
	notificationHandler := handler.NewNotificationHandler(notificationRepo)

	return router.NewRouter(
		router.ServerConfig{ReadTimeout: time.Second * 15, WriteTimeout: time.Second * 15, IdleTimeout: time.Minute},
		tokenIssuer,
		authHandler, activityHandler, metricHandler, insightHandler, stravaHandler, deviceHandler, notificationHandler,
	)
}

// doJSON envia uma requisição com corpo JSON opcional e bearer token opcional,
// e devolve status + corpo decodificado.
func doJSON(t *testing.T, app *fiber.App, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := app.Test(req, 10000) // timeout em ms
	if err != nil {
		t.Fatalf("app.Test %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp.StatusCode, out
}

func TestIntegration_HTTP_FullJourney(t *testing.T) {
	app := buildApp(t)

	suffix := time.Now().Format("150405.000000")
	email := fmt.Sprintf("e2e-%s@example.com", suffix)
	password := "SenhaForte123"
	now := time.Now().UTC()

	// 1) Register.
	status, body := doJSON(t, app, http.MethodPost, "/api/v1/auth/register", "", map[string]any{
		"name":       "E2E User",
		"email":      email,
		"password":   password,
		"birth_date": "1995-03-12T00:00:00Z",
	})
	if status != http.StatusCreated {
		t.Fatalf("register status = %d, want 201; body=%v", status, body)
	}
	if body["id"] == nil || body["id"] == "" {
		t.Fatalf("register did not return id: %v", body)
	}

	// 2) Login -> JWT.
	status, body = doJSON(t, app, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"email":    email,
		"password": password,
	})
	if status != http.StatusOK {
		t.Fatalf("login status = %d, want 200; body=%v", status, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatalf("login did not return token: %v", body)
	}

	// 3) 401 sem token na rota protegida.
	status, _ = doJSON(t, app, http.MethodPost, "/api/v1/insights/generate", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("protected route without token status = %d, want 401", status)
	}

	// 4) POST activity.
	status, body = doJSON(t, app, http.MethodPost, "/api/v1/activities", token, map[string]any{
		"activity_type":    "running",
		"date":             now.Add(-2 * time.Hour).Format(time.RFC3339),
		"duration_minutes": 45,
		"avg_heart_rate":   150,
		"external_id":      "e2e-act-" + suffix,
	})
	if status != http.StatusCreated {
		t.Fatalf("create activity status = %d, want 201; body=%v", status, body)
	}

	// 5) POST metrics (calibrado p/ disparar insights: HRV em queda + sono baixo).
	postMetric := func(mt string, value float64, daysAgo int) {
		st, b := doJSON(t, app, http.MethodPost, "/api/v1/metrics", token, map[string]any{
			"metric_type": mt,
			"value":       value,
			"date":        now.Add(-time.Duration(daysAgo) * 24 * time.Hour).Format(time.RFC3339),
		})
		if st != http.StatusCreated {
			t.Fatalf("create metric %s status = %d, want 201; body=%v", mt, st, b)
		}
	}
	for d := 1; d <= 8; d++ {
		v := 80.0
		if d == 1 {
			v = 55.0
		}
		postMetric("hrv", v, d)
	}

	// 6) Query metrics de volta.
	from := now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	to := now.Add(time.Hour).Format(time.RFC3339)
	status, body = doJSON(t, app, http.MethodGet,
		fmt.Sprintf("/api/v1/metrics?metric_type=hrv&from=%s&to=%s", from, to), token, nil)
	if status != http.StatusOK {
		t.Fatalf("query metrics status = %d, want 200; body=%v", status, body)
	}
	if cnt, _ := body["count"].(float64); cnt != 8 {
		t.Fatalf("query metrics count = %v, want 8", body["count"])
	}

	// 7) Generate insights (>=1 esperado pela queda de HRV).
	status, body = doJSON(t, app, http.MethodPost, "/api/v1/insights/generate", token, nil)
	if status != http.StatusOK {
		t.Fatalf("generate insights status = %d, want 200; body=%v", status, body)
	}
	if cnt, _ := body["count"].(float64); cnt < 1 {
		t.Fatalf("expected at least 1 insight, got %v", body["count"])
	}

	// 8) Register device.
	status, _ = doJSON(t, app, http.MethodPost, "/api/v1/notifications/devices", token, map[string]any{
		"token":    "e2e-device-" + suffix,
		"platform": "android",
	})
	if status != http.StatusNoContent {
		t.Fatalf("register device status = %d, want 204", status)
	}

	// 9) GET notifications (lista vazia, mas 200 e shape correto).
	status, body = doJSON(t, app, http.MethodGet, "/api/v1/notifications", token, nil)
	if status != http.StatusOK {
		t.Fatalf("list notifications status = %d, want 200; body=%v", status, body)
	}
	if _, ok := body["notifications"]; !ok {
		t.Fatalf("notifications response missing 'notifications' field: %v", body)
	}

	// 10) Strava connect retorna redirect 302 com Location.
	st := stravaConnectStatus(t, app, token)
	if st != http.StatusFound {
		t.Fatalf("strava connect status = %d, want 302", st)
	}
}

// stravaConnectStatus chama o connect e devolve o status sem seguir o redirect.
func stravaConnectStatus(t *testing.T, app *fiber.App, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/connectors/strava/connect", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, 10000)
	if err != nil {
		t.Fatalf("strava connect: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
