package main

import (
	"context"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// noopEventPublisher descarta todos os eventos. É um fallback documentado para
// rodar a API sem um broker RabbitMQ (ex.: testes locais, ambientes onde a
// mensageria assíncrona não é necessária). O main usa o publisher real do
// RabbitMQ; troque por &noopEventPublisher{} para desabilitar a publicação.
//
//nolint:unused // mantido intencionalmente como fallback.
type noopEventPublisher struct{}

var _ port.EventPublisher = (*noopEventPublisher)(nil)

func (p *noopEventPublisher) Publish(_ context.Context, _ port.Event) error {
	return nil
}
