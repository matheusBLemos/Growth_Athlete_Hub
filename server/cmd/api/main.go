package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/config"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/insights/deterministic"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"

	router "github.com/Growth-Athlete-Hub/gah-server/internal/infra/http"
)

func main() {
	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := postgres.NewDB(cfg.Database.URL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime.Duration)

	activityRepo := postgres.NewActivityRepository(db)
	metricRepo := postgres.NewMetricRepository(db)
	insightRepo := postgres.NewInsightRepository(db)

	evaluator := deterministic.NewCompositeEvaluator(
		deterministic.NewHRVRule(),
		deterministic.NewRestingHRRule(),
		deterministic.NewSleepRule(),
		deterministic.NewACWRRule(),
		deterministic.NewRecoveryRule(),
	)

	publisher := &noopEventPublisher{}

	registerActivity := usecase.NewRegisterActivity(activityRepo, publisher)
	recordMetric := usecase.NewRecordMetric(metricRepo, publisher)
	queryMetrics := usecase.NewQueryMetrics(metricRepo)
	generateInsights := usecase.NewGenerateInsights(metricRepo, insightRepo, evaluator)

	activityHandler := handler.NewActivityHandler(registerActivity)
	metricHandler := handler.NewMetricHandler(recordMetric, queryMetrics)
	insightHandler := handler.NewInsightHandler(generateInsights)

	r := router.NewRouter(activityHandler, metricHandler, insightHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout.Duration,
		WriteTimeout: cfg.Server.WriteTimeout.Duration,
		IdleTimeout:  cfg.Server.IdleTimeout.Duration,
	}

	go func() {
		fmt.Printf("GAH Server starting on :%d\n", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.WriteTimeout.Duration)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("server stopped")
}
