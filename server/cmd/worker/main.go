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

	registerHandlers(subscriber, db, publisher, cfg.Notifications)

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
func registerHandlers(subscriber *rabbitmq.Subscriber, db *sql.DB, publisher port.EventPublisher, notifyCfg config.NotificationsConfig) {
	activityRepo := postgres.NewActivityRepository(db)
	metricRepo := postgres.NewMetricRepository(db)
	insightRepo := postgres.NewInsightRepository(db)
	aggRepo := postgres.NewAggregatedMetricRepository(db)
	deviceRepo := postgres.NewDeviceRepository(db)

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
	notifyInsight := usecase.NewNotifyInsight(deviceRepo, notifier)
	subscriber.Register(notifications.NewInsightNotificationHandler(notifyInsight))

	// TODO: strava.webhook.activity é apenas um gatilho (athlete/object IDs); o
	// seu handler precisaria do SyncProviderActivities (gateway + token repo)
	// para buscar a atividade e republicar raw.activity.imported. Quando o
	// worker tiver acesso ao gateway/token repo, registre-o aqui. Por ora, o
	// fluxo de sync é disparado pela API (StravaHandler).
}

// buildNotifier escolhe o adaptador de push conforme a config: FCMNotifier
// quando provider="fcm" e há server key; caso contrário o LogNotifier (stub
// seguro, sem chamada externa).
func buildNotifier(cfg config.NotificationsConfig) port.Notifier {
	if cfg.Provider == "fcm" && cfg.FCMServerKey != "" {
		log.Println("notifications: using FCM push notifier")
		return notifications.NewFCMNotifier(notifications.FCMConfig{
			BaseURL:   cfg.FCMBaseURL,
			ServerKey: cfg.FCMServerKey,
		})
	}
	log.Println("notifications: using log notifier (no push provider configured)")
	return notifications.NewLogNotifier()
}
