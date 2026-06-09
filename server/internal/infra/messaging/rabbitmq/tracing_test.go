package rabbitmq

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestTracePropagationRoundTrip garante que o contexto de trace injetado nos
// headers AMQP pelo publisher é recuperado pelo subscriber — base do trace
// distribuído API -> worker.
func TestTracePropagationRoundTrip(t *testing.T) {
	// O round-trip depende do propagador global (instalado por observability.Setup
	// em produção). No teste, instalamos o W3C TraceContext diretamente.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	tid, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	sid, _ := trace.SpanIDFromHex("1112131415161718")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	producerCtx := trace.ContextWithSpanContext(context.Background(), sc)

	// Publisher injeta o contexto nos headers.
	headers := injectTrace(producerCtx, amqp.Table{})
	if len(headers) == 0 {
		t.Fatal("injectTrace produced no headers; W3C traceparent expected")
	}

	// Consumer (sem contexto prévio) extrai dos headers.
	consumerCtx := extractTrace(context.Background(), headers)
	got := trace.SpanContextFromContext(consumerCtx)

	if !got.IsValid() {
		t.Fatal("extracted span context is invalid")
	}
	if got.TraceID() != tid {
		t.Errorf("trace id = %s, want %s", got.TraceID(), tid)
	}
	if got.SpanID() != sid {
		t.Errorf("span id = %s, want %s", got.SpanID(), sid)
	}
}

// TestExtractTrace_NilHeaders é seguro e devolve o ctx original.
func TestExtractTrace_NilHeaders(t *testing.T) {
	ctx := context.Background()
	if got := extractTrace(ctx, nil); got != ctx {
		t.Error("extractTrace(nil headers) should return the original context")
	}
}

// TestAMQPHeaderCarrier exercita o adapter de carrier diretamente.
func TestAMQPHeaderCarrier(t *testing.T) {
	c := amqpHeaderCarrier{}
	c.Set("k", "v")
	if got := c.Get("k"); got != "v" {
		t.Errorf("Get(k) = %q, want v", got)
	}
	if got := c.Get("missing"); got != "" {
		t.Errorf("Get(missing) = %q, want empty", got)
	}
	// Valor não-string é ignorado (headers AMQP são heterogêneos).
	c["n"] = 42
	if got := c.Get("n"); got != "" {
		t.Errorf("Get(non-string) = %q, want empty", got)
	}
	if keys := c.Keys(); len(keys) != 2 {
		t.Errorf("Keys() len = %d, want 2", len(keys))
	}
}
