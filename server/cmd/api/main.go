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
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/observability"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/observability/logging"
	obsmetrics "github.com/Growth-Athlete-Hub/gah-server/internal/infra/observability/metrics"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"

	router "github.com/Growth-Athlete-Hub/gah-server/internal/infra/http"
)

func main() {
	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Logger estruturado (JSON em stdout) usado por toda a aplicação.
	logger := logging.New(os.Stdout, logging.ParseLevel(cfg.Observability.LogLevel))
	ctx := context.Background()

	// fatal loga o erro de boot de forma estruturada e encerra o processo.
	fatal := func(msg string, err error) {
		logger.Error(ctx, msg, "error", err)
		os.Exit(1)
	}

	// Telemetria OpenTelemetry (traces + métricas). No-op quando desabilitada.
	obsCfg := observability.Config{
		Enabled:        cfg.Observability.Enabled,
		ServiceName:    observability.ServiceName(cfg.Observability.ServiceName, "gah-api"),
		ServiceVersion: cfg.Observability.ServiceVersion,
		Environment:    cfg.Observability.Environment,
		OTLPEndpoint:   cfg.Observability.OTLPEndpoint,
		SampleRatio:    cfg.Observability.SampleRatio,
		Insecure:       cfg.Observability.Insecure,
	}
	shutdownObs, err := observability.Setup(ctx, obsCfg)
	if err != nil {
		fatal("failed to set up observability", err)
	}
	defer func() { _ = shutdownObs(context.Background()) }()

	// Criado após o Setup para capturar o meter global já configurado.
	metrics := obsmetrics.New()

	db, err := postgres.NewDB(cfg.Database.URL)
	if err != nil {
		fatal("failed to connect to database", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime.Duration)
	db.SetConnMaxIdleTime(cfg.Database.ConnMaxIdleTime.Duration)

	// Auto-migração no boot: aplica as migrations embutidas (rastreadas em
	// schema_migrations) antes de servir. Idempotente.
	if err := postgres.Migrate(db); err != nil {
		fatal("failed to run migrations", err)
	}

	activityRepo := postgres.NewActivityRepository(db)
	metricRepo := postgres.NewMetricRepository(db)
	insightRepo := postgres.NewInsightRepository(db)
	userRepo := postgres.NewUserRepository(db)
	providerTokenRepo := postgres.NewProviderTokenRepository(db)
	deviceRepo := postgres.NewDeviceRepository(db)
	notificationRepo := postgres.NewNotificationRepository(db)

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
		fatal("failed to connect to rabbitmq", err)
	}
	defer publisher.Close()

	cache, err := rediscache.New(rediscache.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		fatal("failed to create redis cache", err)
	}
	if err := cache.Ping(context.Background()); err != nil {
		fatal("failed to connect to redis", err)
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
	notificationHandler := handler.NewNotificationHandler(notificationRepo)

	app := router.NewRouter(
		router.ServerConfig{
			ReadTimeout:  cfg.Server.ReadTimeout.Duration,
			WriteTimeout: cfg.Server.WriteTimeout.Duration,
			IdleTimeout:  cfg.Server.IdleTimeout.Duration,
		},
		logger,
		metrics,
		tokenIssuer,
		authHandler, activityHandler, metricHandler, insightHandler, stravaHandler, deviceHandler, notificationHandler,
	)

	go func() {
		logger.Info(ctx, "GAH Server starting", "port", cfg.Server.Port)
		if err := app.Listen(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
			fatal("server error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.WriteTimeout.Duration)
	defer cancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		fatal("server forced to shutdown", err)
	}
	logger.Info(ctx, "server stopped")
}
