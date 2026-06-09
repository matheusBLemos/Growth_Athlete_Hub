// Package processing contém os consumidores (MessageHandler) do módulo de
// Processamento do GAH. Eles decodificam eventos do broker e despacham para as
// use cases do pipeline (validação -> dedup -> normalização -> persistência ->
// agregação -> insights).
package processing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/messaging/rabbitmq"
)

// rawActivityProcessor é o seam para a use case ProcessRawActivity.
// *usecase.ProcessRawActivity o satisfaz.
type rawActivityProcessor interface {
	Execute(ctx context.Context, raw usecase.RawActivityImported) error
}

// RawActivityHandler consome eventos raw.activity.imported, decodifica o payload
// para usecase.RawActivityImported e o despacha para o pipeline de
// processamento. Implementa rabbitmq.MessageHandler.
type RawActivityHandler struct {
	processor rawActivityProcessor
}

func NewRawActivityHandler(processor rawActivityProcessor) *RawActivityHandler {
	return &RawActivityHandler{processor: processor}
}

func (h *RawActivityHandler) EventType() string {
	return usecase.RawActivityImportedEventType
}

func (h *RawActivityHandler) Handle(ctx context.Context, event port.Event) error {
	raw, err := decodePayload[usecase.RawActivityImported](event.Payload)
	if err != nil {
		return fmt.Errorf("decode %s payload: %w", h.EventType(), err)
	}
	return h.processor.Execute(ctx, raw)
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
	_ rabbitmq.MessageHandler = (*RawActivityHandler)(nil)
	_ rawActivityProcessor    = (*usecase.ProcessRawActivity)(nil)
)
