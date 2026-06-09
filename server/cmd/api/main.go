package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/auth"
	rediscache "github.com/Growth-Athlete-Hub/gah-server/internal/infra/cache/redis"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/config"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/connectors/strava"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/insights/deterministic"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/messaging/rabbitmq"
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
	userRepo := postgres.NewUserRepository(db)
	providerTokenRepo := postgres.NewProviderTokenRepository(db)
	deviceRepo := postgres.NewDeviceRepository(db)

	hasher := auth.NewArgon2Hasher(cfg.Auth.PasswordPepper)
	tokenIssuer := auth.NewJWTIssuer(cfg.Auth.JWTSecret, cfg.Auth.TokenTTL.Duration)

	evaluator := deterministic.NewCompositeEvaluator(
		deterministic.NewHRVRule(),
		deterministic.NewRestingHRRule(),
		deterministic.NewSleepRule(),
		deterministic.NewACWRRule(),
		deterministic.NewRecoveryRule(),
	)

	publisher, err := rabbitmq.NewPublisher(cfg.Messaging.URL, cfg.Messaging.Exchange)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer publisher.Close()

	cache, err := rediscache.New(rediscache.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		log.Fatalf("failed to create redis cache: %v", err)
	}
	if err := cache.Ping(context.Background()); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	defer cache.Close()

	metricsTTL := cfg.Redis.MetricsTTL.Duration

	registerActivity := usecase.NewRegisterActivity(activityRepo, publisher)
	recordMetric := usecase.NewRecordMetric(metricRepo, publisher, cache, metricsTTL)
	queryMetrics := usecase.NewQueryMetrics(metricRepo, cache, metricsTTL)
	generateInsights := usecase.NewGenerateInsights(metricRepo, insightRepo, evaluator)
	registerUser := usecase.NewRegisterUser(userRepo, hasher, publisher)
	loginUser := usecase.NewLoginUser(userRepo, hasher, tokenIssuer)

	stravaGateway := strava.NewGateway(strava.Config{
		ClientID:     cfg.Connectors.Strava.ClientID,
		ClientSecret: cfg.Connectors.Strava.ClientSecret,
		RedirectURL:  cfg.Connectors.Strava.RedirectURL,
		AuthURL:      cfg.Connectors.Strava.AuthURL,
		TokenURL:     cfg.Connectors.Strava.TokenURL,
		APIBaseURL:   cfg.Connectors.Strava.APIBaseURL,
	})
	connectProvider := usecase.NewConnectProvider(stravaGateway, providerTokenRepo)
	syncProvider := usecase.NewSyncProviderActivities(stravaGateway, providerTokenRepo, publisher)

	authHandler := handler.NewAuthHandler(registerUser, loginUser)
	activityHandler := handler.NewActivityHandler(registerActivity)
	metricHandler := handler.NewMetricHandler(recordMetric, queryMetrics)
	insightHandler := handler.NewInsightHandler(generateInsights)
	stravaHandler := handler.NewStravaHandler(connectProvider, syncProvider, publisher, tokenIssuer, cfg.Connectors.Strava.WebhookVerifyToken)
	deviceHandler := handler.NewDeviceHandler(deviceRepo)

	app := router.NewRouter(
		router.ServerConfig{
			ReadTimeout:  cfg.Server.ReadTimeout.Duration,
			WriteTimeout: cfg.Server.WriteTimeout.Duration,
			IdleTimeout:  cfg.Server.IdleTimeout.Duration,
		},
		tokenIssuer,
		authHandler, activityHandler, metricHandler, insightHandler, stravaHandler, deviceHandler,
	)

	go func() {
		fmt.Printf("GAH Server starting on :%d\n", cfg.Server.Port)
		if err := app.Listen(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.WriteTimeout.Duration)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("server stopped")
}
