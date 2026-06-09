package http

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/middleware"
)

// ServerConfig carrega os parâmetros de transporte do servidor HTTP.
// Mantém o pacote http desacoplado do pacote de configuração.
type ServerConfig struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func NewRouter(
	cfg ServerConfig,
	tokenIssuer port.TokenIssuer,
	authHandler *handler.AuthHandler,
	activityHandler *handler.ActivityHandler,
	metricHandler *handler.MetricHandler,
	insightHandler *handler.InsightHandler,
	stravaHandler *handler.StravaHandler,
	deviceHandler *handler.DeviceHandler,
) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit:    1 << 20, // 1 MiB
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	})

	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	v1 := app.Group("/api/v1")

	// Rotas públicas de autenticação.
	v1.Post("/auth/register", authHandler.Register)
	v1.Post("/auth/login", authHandler.Login)

	// Rotas públicas do conector Strava: callback OAuth e webhook são chamados
	// pela própria Strava (sem o token de auth do GAH).
	v1.Get("/connectors/strava/callback", stravaHandler.Callback)
	v1.Get("/connectors/strava/webhook", stravaHandler.WebhookVerify)
	v1.Post("/connectors/strava/webhook", stravaHandler.WebhookEvent)

	// Rotas protegidas: exigem token válido. O userID é injetado em c.Locals.
	protected := v1.Group("", middleware.Auth(tokenIssuer))

	protected.Post("/activities", activityHandler.Register)

	protected.Post("/metrics", metricHandler.Record)
	protected.Get("/metrics", metricHandler.Query)

	protected.Post("/insights/generate", insightHandler.Generate)

	// Registro de dispositivos para notificações push do usuário autenticado.
	protected.Post("/notifications/devices", deviceHandler.Register)

	// Conexão e sync exigem usuário autenticado.
	protected.Get("/connectors/strava/connect", stravaHandler.Connect)
	protected.Post("/connectors/strava/sync", stravaHandler.Sync)

	return app
}
