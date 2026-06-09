package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// MessageHandler é o contrato que módulos futuros (processamento, notificações)
// implementam para reagir a um tipo de evento. EventType retorna a routing key
// (ex.: "metric.recorded") à qual a fila do consumidor será vinculada.
type MessageHandler interface {
	EventType() string
	Handle(ctx context.Context, event port.Event) error
}

// HandlerFunc adapta uma função simples ao contrato MessageHandler, para uso
// com Subscriber.Register sem precisar declarar um tipo.
type HandlerFunc struct {
	Type string
	Fn   func(ctx context.Context, event port.Event) error
}

func (h HandlerFunc) EventType() string { return h.Type }

func (h HandlerFunc) Handle(ctx context.Context, event port.Event) error {
	return h.Fn(ctx, event)
}

// delivery abstrai o que o dispatcher precisa de uma amqp091.Delivery,
// permitindo testar a lógica de decode/ack/nack com entregas falsas.
type delivery interface {
	Body() []byte
	RoutingKey() string
	Headers() amqp.Table
	Ack() error
	Nack(requeue bool) error
}

// amqpDelivery adapta uma amqp.Delivery real à interface delivery.
type amqpDelivery struct {
	d amqp.Delivery
}

func (a amqpDelivery) Body() []byte        { return a.d.Body }
func (a amqpDelivery) RoutingKey() string  { return a.d.RoutingKey }
func (a amqpDelivery) Headers() amqp.Table { return a.d.Headers }
func (a amqpDelivery) Ack() error          { return a.d.Ack(false) }
func (a amqpDelivery) Nack(requeue bool) error {
	return a.d.Nack(false, requeue)
}

// subChannel abstrai as operações de canal AMQP usadas pelo Subscriber, para
// permitir testar a montagem (declare/bind) sem broker vivo.
type subChannel interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Qos(prefetchCount, prefetchSize int, global bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	Close() error
}

// Subscriber declara uma fila por handler, vincula cada uma ao topic exchange
// pela routing key do handler e consome as entregas, despachando e dando
// ack/nack conforme o resultado. É o seam de consumo de eventos do GAH.
type Subscriber struct {
	conn        *amqp.Connection
	channel     subChannel
	exchange    string
	queuePrefix string
	prefetch    int

	handlers map[string]MessageHandler
}

// NewSubscriber disca no RabbitMQ, abre um canal, declara o topic exchange e
// aplica o QoS de prefetch. Os handlers são registrados via Register antes de
// chamar Start. Use Close no shutdown.
func NewSubscriber(url, exchange, queuePrefix string, prefetch int) (*Subscriber, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open channel: %w", err)
	}

	sub, err := newSubscriberWithChannel(ch, exchange, queuePrefix, prefetch)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}
	sub.conn = conn
	return sub, nil
}

func newSubscriberWithChannel(ch subChannel, exchange, queuePrefix string, prefetch int) (*Subscriber, error) {
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

	if err := ch.Qos(prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("set qos: %w", err)
	}

	return &Subscriber{
		channel:     ch,
		exchange:    exchange,
		queuePrefix: queuePrefix,
		prefetch:    prefetch,
		handlers:    make(map[string]MessageHandler),
	}, nil
}

// Register associa um handler à sua routing key (handler.EventType()).
// Registrar dois handlers para o mesmo tipo sobrescreve o anterior.
func (s *Subscriber) Register(h MessageHandler) {
	s.handlers[h.EventType()] = h
}

// RegisterFunc é um atalho para registrar uma função como handler de um tipo.
func (s *Subscriber) RegisterFunc(eventType string, fn func(ctx context.Context, event port.Event) error) {
	s.Register(HandlerFunc{Type: eventType, Fn: fn})
}

// Start declara e vincula uma fila durável por handler registrado, inicia o
// consumo de cada uma e bloqueia até ctx ser cancelado. Cada fila usa uma
// dead-letter exchange ("<exchange>.dlx") para mensagens que falham, evitando
// requeue infinito.
func (s *Subscriber) Start(ctx context.Context) error {
	if len(s.handlers) == 0 {
		return fmt.Errorf("no handlers registered")
	}

	// Dead-letter exchange: mensagens nack-adas sem requeue caem aqui.
	dlx := s.exchange + ".dlx"
	if err := s.channel.ExchangeDeclare(dlx, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlx: %w", err)
	}

	for eventType, handler := range s.handlers {
		queueName := s.queuePrefix + "." + eventType
		args := amqp.Table{"x-dead-letter-exchange": dlx}

		if _, err := s.channel.QueueDeclare(queueName, true, false, false, false, args); err != nil {
			return fmt.Errorf("declare queue %q: %w", queueName, err)
		}
		if err := s.channel.QueueBind(queueName, eventType, s.exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue %q: %w", queueName, err)
		}

		// Dead-letter queue durável vinculada à DLX pela mesma routing key.
		dlq := queueName + ".dead"
		if _, err := s.channel.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare dlq %q: %w", dlq, err)
		}
		if err := s.channel.QueueBind(dlq, eventType, dlx, false, nil); err != nil {
			return fmt.Errorf("bind dlq %q: %w", dlq, err)
		}

		deliveries, err := s.channel.Consume(queueName, "", false, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("consume queue %q: %w", queueName, err)
		}

		go s.consume(ctx, handler, deliveries)
	}

	<-ctx.Done()
	return nil
}

// consume recebe entregas reais e as adapta para o dispatcher testável.
func (s *Subscriber) consume(ctx context.Context, handler MessageHandler, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			s.dispatch(ctx, handler, amqpDelivery{d: d})
		}
	}
}

// dispatch decodifica o corpo JSON da entrega em port.Event, invoca o handler
// e dá ack em caso de sucesso ou nack (sem requeue -> dead-letter) em caso de
// falha de decode ou de processamento. Mantida pequena e sem dependência de
// AMQP vivo para ser testada com entregas falsas.
func (s *Subscriber) dispatch(ctx context.Context, handler MessageHandler, d delivery) {
	// Continua o trace distribuído iniciado no publisher: extrai o contexto dos
	// headers e abre um span de consumer. Com a telemetria off, é no-op.
	ctx = extractTrace(ctx, d.Headers())
	ctx, span := otel.Tracer(tracerName).Start(ctx, d.RoutingKey()+" process",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			semconv.MessagingSystemRabbitmq,
			semconv.MessagingDestinationName(s.exchange),
		),
	)
	defer span.End()

	logger := port.LoggerFromContext(ctx)

	var event port.Event
	if err := json.Unmarshal(d.Body(), &event); err != nil {
		// Mensagem malformada nunca vai melhorar com requeue -> dead-letter.
		logger.Error(ctx, "rabbitmq: decode delivery failed", "routing_key", d.RoutingKey(), "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "decode failed")
		_ = d.Nack(false)
		return
	}

	if err := handler.Handle(ctx, event); err != nil {
		logger.Error(ctx, "rabbitmq: handle event failed", "event", event.Type, "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "handler failed")
		// Sem requeue: roteia para a DLX para inspeção/retry controlado,
		// evitando loop de reprocessamento de mensagens "envenenadas".
		_ = d.Nack(false)
		return
	}

	if err := d.Ack(); err != nil {
		logger.Error(ctx, "rabbitmq: ack event failed", "event", event.Type, "error", err)
		span.RecordError(err)
	}
}

// Close fecha o canal e a conexão.
func (s *Subscriber) Close() error {
	if s.channel != nil {
		_ = s.channel.Close()
	}
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
