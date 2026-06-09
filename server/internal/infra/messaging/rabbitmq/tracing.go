package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// tracerName identifica o instrumentation scope dos spans de mensageria.
const tracerName = "github.com/Growth-Athlete-Hub/gah-server/internal/infra/messaging/rabbitmq"

// amqpHeaderCarrier adapta uma amqp.Table ao contrato propagation.TextMapCarrier,
// permitindo que o propagador OpenTelemetry injete/extraia o contexto de trace
// dos headers AMQP — base do trace distribuído entre publisher e consumer.
type amqpHeaderCarrier amqp.Table

var _ propagation.TextMapCarrier = (amqpHeaderCarrier)(nil)

func (c amqpHeaderCarrier) Get(key string) string {
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (c amqpHeaderCarrier) Set(key, value string) {
	c[key] = value
}

func (c amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// injectTrace serializa o contexto de trace do ctx nos headers AMQP, criando o
// mapa quando necessário, e devolve os headers prontos para a publicação.
func injectTrace(ctx context.Context, headers amqp.Table) amqp.Table {
	if headers == nil {
		headers = amqp.Table{}
	}
	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(headers))
	return headers
}

// extractTrace recupera o contexto de trace propagado nos headers AMQP, ligando
// o consumer ao trace que originou a mensagem no publisher.
func extractTrace(ctx context.Context, headers amqp.Table) context.Context {
	if headers == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, amqpHeaderCarrier(headers))
}
