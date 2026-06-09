package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/insights/deterministic"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"

	router "github.com/Growth-Athlete-Hub/gah-server/internal/infra/http"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/gah?sslmode=disable"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := postgres.NewDB(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

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

	fmt.Printf("GAH Server starting on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
