package notifications

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/messaging/rabbitmq"
)

// insightNotifier é o seam para a use case NotifyInsight.
// *usecase.NotifyInsight o satisfaz.
type insightNotifier interface {
	Execute(ctx context.Context, insight usecase.InsightGenerated) error
}

// InsightNotificationHandler consome eventos insight.generated, decodifica o
// payload para usecase.InsightGenerated e o despacha para a use case de
// notificação. Implementa rabbitmq.MessageHandler.
type InsightNotificationHandler struct {
	notifier insightNotifier
}

func NewInsightNotificationHandler(notifier insightNotifier) *InsightNotificationHandler {
	return &InsightNotificationHandler{notifier: notifier}
}

func (h *InsightNotificationHandler) EventType() string {
	return usecase.InsightGeneratedEventType
}

func (h *InsightNotificationHandler) Handle(ctx context.Context, event port.Event) error {
	insight, err := decodePayload[usecase.InsightGenerated](event.Payload)
	if err != nil {
		return fmt.Errorf("decode %s payload: %w", h.EventType(), err)
	}
	return h.notifier.Execute(ctx, insight)
}

// decodePayload converte um Event.Payload genérico no tipo T. No lado do
// consumidor, o subscriber já fez json.Unmarshal do envelope, então Payload
// costuma vir como map[string]any; in-process pode vir tipado. Re-serializar e
// deserializar normaliza ambos os casos de forma robusta.
func decodePayload[T any](payload any) (T, error) {
	var out T
	if typed, ok := payload.(T); ok {
		return typed, nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	return out, nil
}

var (
	_ rabbitmq.MessageHandler = (*InsightNotificationHandler)(nil)
	_ insightNotifier         = (*usecase.NotifyInsight)(nil)
)
