// Package rabbitmq implementa a camada de mensageria assíncrona do GAH sobre
// RabbitMQ (AMQP 0-9-1). O publisher implementa a porta port.EventPublisher e
// o subscriber consome eventos de um topic exchange, despachando para handlers
// registrados — é o ponto de extensão (seam) para módulos futuros de
// processamento e notificações.
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// contentTypeJSON é o content-type usado em todas as mensagens publicadas.
const contentTypeJSON = "application/json"

// amqpChannel abstrai as operações de canal AMQP usadas pelo Publisher.
// Existe para permitir testes unitários com um canal falso, sem broker vivo.
type amqpChannel interface {
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	Close() error
}

// Publisher publica eventos do domínio num topic exchange do RabbitMQ.
// A routing key de cada mensagem é o event.Type e o corpo é o JSON do payload.
type Publisher struct {
	conn     *amqp.Connection
	channel  amqpChannel
	exchange string
}

var _ port.EventPublisher = (*Publisher)(nil)

// NewPublisher disca no RabbitMQ, abre um canal e declara o topic exchange
// (durável). Use Close para liberar canal e conexão no shutdown.
func NewPublisher(url, exchange string) (*Publisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	pub, err := newPublisherWithChannel(ch, exchange)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	pub.conn = conn
	return pub, nil
}

// newPublisherWithChannel constrói o Publisher a partir de um canal já aberto e
// declara o exchange. Usado pelo construtor real e pelos testes com fake.
func newPublisherWithChannel(ch amqpChannel, exchange string) (*Publisher, error) {
	if err := ch.ExchangeDeclare(
		exchange,
		amqp.ExchangeTopic,
		true,  // durable
		false, // autoDelete
		false, // internal
		false, // noWait
		nil,
	); err != nil {
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	return &Publisher{
		channel:  ch,
		exchange: exchange,
	}, nil
}

// Publish serializa event.Payload em JSON e publica no topic exchange usando
// event.Type como routing key, com entrega persistente.
func (p *Publisher) Publish(ctx context.Context, event port.Event) error {
	body, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	if err := p.channel.PublishWithContext(
		ctx,
		p.exchange,
		event.Type, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:  contentTypeJSON,
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	); err != nil {
		return fmt.Errorf("publish event %q: %w", event.Type, err)
	}
	return nil
}

// Close fecha o canal e a conexão, ignorando erros de recursos já fechados.
func (p *Publisher) Close() error {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}
