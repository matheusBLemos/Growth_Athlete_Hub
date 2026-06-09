// Command worker é o consumidor de eventos assíncronos do GAH. Ele conecta ao
// RabbitMQ, registra os handlers de eventos e processa as mensagens publicadas
// pela API (via port.EventPublisher) até receber SIGINT/SIGTERM.
//
// Hoje registra apenas um handler de log como placeholder, para que o worker
// seja executável. Módulos futuros (processamento de métricas, geração de
// insights, notificações) registram seus handlers aqui — ver comentário em
// registerHandlers.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/config"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/messaging/rabbitmq"
)

func main() {
	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	subscriber, err := rabbitmq.NewSubscriber(
		cfg.Messaging.URL,
		cfg.Messaging.Exchange,
		cfg.Messaging.QueuePrefix,
		cfg.Messaging.Prefetch,
	)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer subscriber.Close()

	registerHandlers(subscriber)

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

// registerHandlers registra os handlers de eventos no subscriber.
//
// Por enquanto há apenas um handler de log (placeholder) para os eventos já
// emitidos pela API. Quando os módulos de processamento/notificação existirem,
// registre seus handlers aqui, por exemplo:
//
//	subscriber.Register(processing.NewMetricRecordedHandler(...))
//	subscriber.RegisterFunc("activity.registered", notif.OnActivity)
func registerHandlers(subscriber *rabbitmq.Subscriber) {
	logHandler := func(_ context.Context, event port.Event) error {
		log.Printf("event received: type=%s payload=%v", event.Type, event.Payload)
		return nil
	}

	subscriber.RegisterFunc("activity.registered", logHandler)
	subscriber.RegisterFunc("metric.recorded", logHandler)
	subscriber.RegisterFunc("user.registered", logHandler)
}
