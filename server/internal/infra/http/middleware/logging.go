package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// RequestLogger injeta o Logger e o Metrics no UserContext da requisição (para
// que handlers e use cases os recuperem via port.LoggerFromContext /
// port.MetricsFromContext) e emite um log estruturado por request concluída,
// com método, rota, status e latência.
//
// Deve ser registrado DEPOIS do middleware de tracing (otelfiber), para que o
// span já esteja no UserContext e os logs saiam correlacionados ao trace.
func RequestLogger(logger port.Logger, metrics port.Metrics) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := port.ContextWithLogger(c.UserContext(), logger)
		ctx = port.ContextWithMetrics(ctx, metrics)
		c.SetUserContext(ctx)

		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		status := c.Response().StatusCode()
		attrs := []any{
			"method", c.Method(),
			"path", c.Path(),
			"route", c.Route().Path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"ip", c.IP(),
		}
		if err != nil {
			attrs = append(attrs, "error", err.Error())
		}

		switch {
		case status >= 500 || err != nil:
			logger.Error(ctx, "http request", attrs...)
		case status >= 400:
			logger.Warn(ctx, "http request", attrs...)
		default:
			logger.Info(ctx, "http request", attrs...)
		}
		return err
	}
}
