package processing

import (
	"context"
	"fmt"
	"log"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/messaging/rabbitmq"
)

const stravaProvider = "strava"

// providerSyncer é o seam para a use case SyncProviderActivities.
// *usecase.SyncProviderActivities o satisfaz.
type providerSyncer interface {
	Execute(ctx context.Context, input usecase.SyncProviderActivitiesInput) (*usecase.SyncProviderActivitiesOutput, error)
}

// athleteResolver resolve o GAH userID a partir do athlete id do provedor.
// port.ProviderTokenRepository o satisfaz.
type athleteResolver interface {
	FindUserByAthlete(ctx context.Context, provider, athleteID string) (userID string, found bool, err error)
}

// stravaWebhookPayload é a forma do payload publicado pelo StravaHandler em
// strava.webhook.activity. owner_id é o athlete id da Strava (não o GAH userID).
// Os IDs numéricos chegam como número JSON; usamos json.Number para aceitar
// tanto inteiros quanto a forma float64 do lado do consumidor.
type stravaWebhookPayload struct {
	Provider   string `json:"provider"`
	ObjectType string `json:"object_type"`
	AspectType string `json:"aspect_type"`
	ObjectID   any    `json:"object_id"`
	OwnerID    any    `json:"owner_id"`
	EventTime  any    `json:"event_time"`
}

// StravaWebhookHandler consome eventos strava.webhook.activity, resolve o GAH
// userID dono da atividade pelo athlete id e dispara o sync do provedor, que
// por sua vez republica raw.activity.imported para o pipeline de processamento.
// Implementa rabbitmq.MessageHandler.
type StravaWebhookHandler struct {
	resolver athleteResolver
	syncer   providerSyncer
}

func NewStravaWebhookHandler(resolver athleteResolver, syncer providerSyncer) *StravaWebhookHandler {
	return &StravaWebhookHandler{resolver: resolver, syncer: syncer}
}

// EventType retorna o mesmo Type publicado pelo StravaHandler.
func (h *StravaWebhookHandler) EventType() string {
	return handler.StravaWebhookEventType
}

func (h *StravaWebhookHandler) Handle(ctx context.Context, event port.Event) error {
	payload, err := decodePayload[stravaWebhookPayload](event.Payload)
	if err != nil {
		return fmt.Errorf("decode %s payload: %w", h.EventType(), err)
	}

	// Só nos interessam atividades criadas/atualizadas; demais eventos
	// (athlete, deauthorize, delete) viram no-op com ack.
	if payload.ObjectType != "activity" || (payload.AspectType != "create" && payload.AspectType != "update") {
		return nil
	}

	ownerID := anyToString(payload.OwnerID)
	if ownerID == "" {
		log.Printf("strava webhook: evento de atividade sem owner_id, ignorando")
		return nil
	}

	userID, found, err := h.resolver.FindUserByAthlete(ctx, stravaProvider, ownerID)
	if err != nil {
		return fmt.Errorf("resolve athlete %s -> user: %w", ownerID, err)
	}
	if !found {
		// Atleta desconhecido (ex.: nunca conectou pelo GAH). Ack para não
		// reenviar infinitamente uma mensagem que nunca terá usuário.
		log.Printf("strava webhook: athlete %s sem usuário GAH vinculado, ignorando", ownerID)
		return nil
	}

	if _, err := h.syncer.Execute(ctx, usecase.SyncProviderActivitiesInput{UserID: userID, Provider: stravaProvider}); err != nil {
		// Erros reais (sync/repo) propagam -> nack -> dead-letter.
		return fmt.Errorf("sync strava activities for user %s: %w", userID, err)
	}

	return nil
}

// anyToString normaliza um id que pode chegar como string, json.Number,
// float64 (número JSON do lado do consumidor) ou int64 (in-process).
func anyToString(v any) string {
	switch n := v.(type) {
	case nil:
		return ""
	case string:
		return n
	case float64:
		return fmt.Sprintf("%d", int64(n))
	case int64:
		return fmt.Sprintf("%d", n)
	case int:
		return fmt.Sprintf("%d", n)
	default:
		return fmt.Sprintf("%v", n)
	}
}

var (
	_ rabbitmq.MessageHandler = (*StravaWebhookHandler)(nil)
	_ providerSyncer          = (*usecase.SyncProviderActivities)(nil)
	_ athleteResolver         = (port.ProviderTokenRepository)(nil)
)
