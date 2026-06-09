package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
)

func NewRouter(
	activityHandler *handler.ActivityHandler,
	metricHandler *handler.MetricHandler,
	insightHandler *handler.InsightHandler,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/activities", activityHandler.Register)

		r.Post("/metrics", metricHandler.Record)
		r.Get("/metrics", metricHandler.Query)

		r.Post("/insights/generate", insightHandler.Generate)
	})

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	return r
}
