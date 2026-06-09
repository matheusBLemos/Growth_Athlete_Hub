//go:build integration

// Teste de integração da mensageria contra um RabbitMQ REAL. Gated em
// TEST_RABBITMQ_URL — pula quando a env não está configurada. Faz o round-trip
// ponta a ponta: publica via Publisher e consome via Subscriber com um handler
// registrado, verificando que o handler recebeu o evento decodificado. Usa um
// exchange/queue prefix ÚNICO por execução para isolar e limpar.
package rabbitmq_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/messaging/rabbitmq"
)

func TestIntegration_RabbitMQ_PublishConsumeRoundTrip(t *testing.T) {
	url := os.Getenv("TEST_RABBITMQ_URL")
	if url == "" {
		t.Skip("TEST_RABBITMQ_URL not set; skipping RabbitMQ integration test")
	}

	// Prefixos únicos por execução -> sem colisão entre runs/suites.
	suffix := time.Now().Format("150405.000000")
	exchange := "gahtest.events." + suffix
	queuePrefix := "gahtest." + suffix
	eventType := "metric.recorded"

	pub, err := rabbitmq.NewPublisher(url, exchange)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	t.Cleanup(func() { _ = pub.Close() })

	sub, err := rabbitmq.NewSubscriber(url, exchange, queuePrefix, 10)
	if err != nil {
		t.Fatalf("new subscriber: %v", err)
	}
	t.Cleanup(func() { _ = sub.Close() })

	received := make(chan port.Event, 1)
	sub.RegisterFunc(eventType, func(_ context.Context, e port.Event) error {
		received <- e
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Start bloqueia até o ctx cancelar; roda em goroutine.
	go func() { _ = sub.Start(ctx) }()

	// Dá um tempo para o consumidor declarar/bindar a fila antes de publicar.
	time.Sleep(500 * time.Millisecond)

	// O Publisher serializa o envelope port.Event completo como corpo e usa
	// event.Type como routing key, de modo que Type/Payload sobrevivam ao decode
	// no consumidor (mesmo contrato dos handlers reais, ex.: RawActivityHandler
	// via decodePayload).
	if err := pub.Publish(ctx, port.Event{Type: eventType, Payload: map[string]any{"user_id": "u-123", "value": 62.0}}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-received:
		// O consumidor decodificou o envelope publicado de volta num port.Event.
		if got.Type != eventType {
			t.Fatalf("event type mismatch: got %q want %q", got.Type, eventType)
		}
		m, ok := got.Payload.(map[string]any)
		if !ok {
			t.Fatalf("payload type = %T, want map[string]any", got.Payload)
		}
		if m["user_id"] != "u-123" {
			t.Fatalf("payload user_id = %v, want u-123", m["user_id"])
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for event to be consumed")
	}
}
