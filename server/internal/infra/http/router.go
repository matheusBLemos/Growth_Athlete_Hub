package http

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
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

	v1.Post("/activities", activityHandler.Register)

	v1.Post("/metrics", metricHandler.Record)
	v1.Get("/metrics", metricHandler.Query)

	v1.Post("/insights/generate", insightHandler.Generate)

	return app
}
