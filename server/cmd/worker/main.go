// Command worker é o consumidor de eventos assíncronos do GAH. Ele conecta ao
// RabbitMQ, registra os handlers de eventos e processa as mensagens publicadas
// pela API/conectores (via port.EventPublisher) até receber SIGINT/SIGTERM.
//
// Hoje hospeda o módulo de Processamento: consome raw.activity.imported e roda
// o pipeline (validação -> dedup -> normalização -> persistência -> agregação
// -> insights), publicando insight.generated para o módulo de Notificações.
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/config"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/connectors/strava"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/insights/deterministic"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/messaging/rabbitmq"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/notifications"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/observability"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/observability/logging"
	obsmetrics "github.com/Growth-Athlete-Hub/gah-server/internal/infra/observability/metrics"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/processing"
)

func main() {
	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := logging.New(os.Stdout, logging.ParseLevel(cfg.Observability.LogLevel))
	baseCtx := context.Background()

	fatal := func(msg string, err error) {
		logger.Error(baseCtx, msg, "error", err)
		os.Exit(1)
	}

	// Telemetria OpenTelemetry (traces + métricas). No-op quando desabilitada.
	shutdownObs, err := observability.Setup(baseCtx, observability.Config{
		Enabled:        cfg.Observability.Enabled,
		ServiceName:    observability.ServiceName(cfg.Observability.ServiceName, "gah-worker"),
		ServiceVersion: cfg.Observability.ServiceVersion,
		Environment:    cfg.Observability.Environment,
		OTLPEndpoint:   cfg.Observability.OTLPEndpoint,
		SampleRatio:    cfg.Observability.SampleRatio,
		Insecure:       cfg.Observability.Insecure,
	})
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
	// schema_migrations) antes de consumir eventos. Idempotente.
	if err := postgres.Migrate(db); err != nil {
		fatal("failed to run migrations", err)
	}

	// O worker publica eventos derivados (ex.: insight.generated), reusando o
	// mesmo topic exchange da API.
	publisher, err := rabbitmq.NewPublisher(cfg.Messaging.URL, cfg.Messaging.Exchange)
	if err != nil {
		fatal("failed to connect to rabbitmq (publisher)", err)
	}
	defer publisher.Close()

	subscriber, err := rabbitmq.NewSubscriber(
		cfg.Messaging.URL,
		cfg.Messaging.Exchange,
		cfg.Messaging.QueuePrefix,
		cfg.Messaging.Prefetch,
	)
	if err != nil {
		fatal("failed to connect to rabbitmq (subscriber)", err)
	}
	defer subscriber.Close()

	registerHandlers(subscriber, db, publisher, logger, cfg.Notifications, cfg.Connectors.Strava)

	// Injeta Logger e Metrics no contexto base: cada delivery deriva dele, então
	// os logs do consumo saem correlacionados ao span e os use cases publicam
	// suas métricas de negócio também no caminho assíncrono.
	obsCtx := port.ContextWithMetrics(port.ContextWithLogger(baseCtx, logger), metrics)
	ctx, cancel := context.WithCancel(obsCtx)
	defer cancel()

	go func() {
		logger.Info(ctx, "GAH Worker starting, consuming events...")
		if err := subscriber.Start(ctx); err != nil {
			fatal("worker error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "shutting down worker...")
	cancel()
	logger.Info(ctx, "worker stopped")
}

// registerHandlers monta as use cases do pipeline de processamento e registra
// seus handlers no subscriber.
//
// O módulo de Processamento reusa RegisterActivity (persistência + dedup por
// external_id) e GenerateInsights, e roda a agregação diária — exatamente as
// use cases da API, evitando duplicação de lógica.
func registerHandlers(subscriber *rabbitmq.Subscriber, db *sql.DB, publisher port.EventPublisher, logger port.Logger, notifyCfg config.NotificationsConfig, stravaCfg config.StravaConfig) {
	activityRepo := postgres.NewActivityRepository(db)
	metricRepo := postgres.NewMetricRepository(db)
	insightRepo := postgres.NewInsightRepository(db)
	aggRepo := postgres.NewAggregatedMetricRepository(db)
	deviceRepo := postgres.NewDeviceRepository(db)
	notificationRepo := postgres.NewNotificationRepository(db)
	providerTokenRepo := postgres.NewProviderTokenRepository(db)

	evaluator := deterministic.NewCompositeEvaluator(
		deterministic.NewHRVRule(),
		deterministic.NewRestingHRRule(),
		deterministic.NewSleepRule(),
		deterministic.NewACWRRule(),
		deterministic.NewRecoveryRule(),
	)

	registerActivity := usecase.NewRegisterActivity(activityRepo, publisher)
	generateInsights := usecase.NewGenerateInsights(metricRepo, insightRepo, evaluator)
	aggregateDaily := usecase.NewAggregateDailyMetrics(metricRepo, aggRepo)

	processRaw := usecase.NewProcessRawActivity(registerActivity, generateInsights, aggregateDaily, publisher)

	subscriber.Register(processing.NewRawActivityHandler(processRaw))

	// Módulo de Notificações: consome insight.generated e dispara push para os
	// dispositivos do usuário. Usa o FCMNotifier quando há server key
	// configurada; caso contrário cai no LogNotifier (stub seguro, sem rede).
	notifier := buildNotifier(logger, notifyCfg)
	notifyInsight := usecase.NewNotifyInsight(deviceRepo, notifier).WithHistory(notificationRepo)
	subscriber.Register(notifications.NewInsightNotificationHandler(notifyInsight))

	// Conector Strava: consome strava.webhook.activity, resolve o atleta -> GAH
	// userID e dispara o SyncProviderActivities, que busca as atividades novas e
	// republica raw.activity.imported de volta no pipeline de processamento.
	stravaGateway := strava.NewGateway(strava.Config{
		ClientID:     stravaCfg.ClientID,
		ClientSecret: stravaCfg.ClientSecret,
		RedirectURL:  stravaCfg.RedirectURL,
		AuthURL:      stravaCfg.AuthURL,
		TokenURL:     stravaCfg.TokenURL,
		APIBaseURL:   stravaCfg.APIBaseURL,
	})
	syncProvider := usecase.NewSyncProviderActivities(stravaGateway, providerTokenRepo, publisher)
	subscriber.Register(processing.NewStravaWebhookHandler(providerTokenRepo, syncProvider))
}

// buildNotifier escolhe o adaptador de push conforme a config: FCMNotifier
// (HTTP v1) quando provider="fcm" e há credentials file + project id; caso
// contrário o LogNotifier (stub seguro, sem chamada externa). Falha ao montar o
// token source da service-account também cai no LogNotifier.
func buildNotifier(logger port.Logger, cfg config.NotificationsConfig) port.Notifier {
	ctx := context.Background()
	if cfg.Provider == "fcm" && cfg.FCMCredentialsFile != "" && cfg.FCMProjectID != "" {
		ts, err := notifications.NewServiceAccountTokenSource(ctx, cfg.FCMCredentialsFile)
		if err != nil {
			logger.Warn(ctx, "notifications: failed to load FCM credentials; falling back to log notifier", "error", err)
			return notifications.NewLogNotifier()
		}
		logger.Info(ctx, "notifications: using FCM HTTP v1 push notifier")
		return notifications.NewFCMNotifier(notifications.FCMConfig{
			BaseURL:     cfg.FCMBaseURL,
			ProjectID:   cfg.FCMProjectID,
			TokenSource: ts,
		})
	}
	logger.Info(ctx, "notifications: using log notifier (no push provider configured)")
	return notifications.NewLogNotifier()
}
