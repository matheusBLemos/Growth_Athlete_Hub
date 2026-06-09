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

	// Rotas protegidas: exigem token válido. O userID é injetado em c.Locals.
	protected := v1.Group("", middleware.Auth(tokenIssuer))

	protected.Post("/activities", activityHandler.Register)

	protected.Post("/metrics", metricHandler.Record)
	protected.Get("/metrics", metricHandler.Query)

	protected.Post("/insights/generate", insightHandler.Generate)

	return app
}
