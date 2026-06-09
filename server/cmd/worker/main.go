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
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/processing"
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

	// Auto-migração no boot: aplica as migrations embutidas (rastreadas em
	// schema_migrations) antes de consumir eventos. Idempotente.
	if err := postgres.Migrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// O worker publica eventos derivados (ex.: insight.generated), reusando o
	// mesmo topic exchange da API.
	publisher, err := rabbitmq.NewPublisher(cfg.Messaging.URL, cfg.Messaging.Exchange)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq (publisher): %v", err)
	}
	defer publisher.Close()

	subscriber, err := rabbitmq.NewSubscriber(
		cfg.Messaging.URL,
		cfg.Messaging.Exchange,
		cfg.Messaging.QueuePrefix,
		cfg.Messaging.Prefetch,
	)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq (subscriber): %v", err)
	}
	defer subscriber.Close()

	registerHandlers(subscriber, db, publisher, cfg.Notifications, cfg.Connectors.Strava)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		log.Println("GAH Worker starting, consuming events...")
		if err := subscriber.Start(ctx); err != nil {
			log.Fatalf("worker error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down worker...")
	cancel()
	log.Println("worker stopped")
}

// registerHandlers monta as use cases do pipeline de processamento e registra
// seus handlers no subscriber.
//
// O módulo de Processamento reusa RegisterActivity (persistência + dedup por
// external_id) e GenerateInsights, e roda a agregação diária — exatamente as
// use cases da API, evitando duplicação de lógica.
func registerHandlers(subscriber *rabbitmq.Subscriber, db *sql.DB, publisher port.EventPublisher, notifyCfg config.NotificationsConfig, stravaCfg config.StravaConfig) {
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
	notifier := buildNotifier(notifyCfg)
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
func buildNotifier(cfg config.NotificationsConfig) port.Notifier {
	if cfg.Provider == "fcm" && cfg.FCMCredentialsFile != "" && cfg.FCMProjectID != "" {
		ts, err := notifications.NewServiceAccountTokenSource(context.Background(), cfg.FCMCredentialsFile)
		if err != nil {
			log.Printf("notifications: failed to load FCM credentials (%v); falling back to log notifier", err)
			return notifications.NewLogNotifier()
		}
		log.Println("notifications: using FCM HTTP v1 push notifier")
		return notifications.NewFCMNotifier(notifications.FCMConfig{
			BaseURL:     cfg.FCMBaseURL,
			ProjectID:   cfg.FCMProjectID,
			TokenSource: ts,
		})
	}
	log.Println("notifications: using log notifier (no push provider configured)")
	return notifications.NewLogNotifier()
}
