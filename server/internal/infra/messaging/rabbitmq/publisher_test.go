package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// fakeChannel é um amqpChannel falso que captura as chamadas para asserção.
type fakeChannel struct {
	declaredExchange string
	declaredKind     string
	declaredDurable  bool

	lastExchange string
	lastKey      string
	lastMsg      amqp.Publishing

	publishErr error
	declareErr error
	closed     bool
}

func (f *fakeChannel) PublishWithContext(_ context.Context, exchange, key string, _, _ bool, msg amqp.Publishing) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	f.lastExchange = exchange
	f.lastKey = key
	f.lastMsg = msg
	return nil
}

func (f *fakeChannel) ExchangeDeclare(name, kind string, durable, _, _, _ bool, _ amqp.Table) error {
	if f.declareErr != nil {
		return f.declareErr
	}
	f.declaredExchange = name
	f.declaredKind = kind
	f.declaredDurable = durable
	return nil
}

func (f *fakeChannel) Close() error {
	f.closed = true
	return nil
}

func newTestPublisher(t *testing.T, ch amqpChannel) *Publisher {
	t.Helper()
	pub, err := newPublisherWithChannel(ch, "gah.events")
	if err != nil {
		t.Fatalf("newPublisherWithChannel: %v", err)
	}
	return pub
}

func TestNewPublisher_DeclaresTopicExchange(t *testing.T) {
	ch := &fakeChannel{}
	newTestPublisher(t, ch)

	if ch.declaredExchange != "gah.events" {
		t.Errorf("exchange = %q, want gah.events", ch.declaredExchange)
	}
	if ch.declaredKind != amqp.ExchangeTopic {
		t.Errorf("kind = %q, want topic", ch.declaredKind)
	}
	if !ch.declaredDurable {
		t.Error("exchange should be durable")
	}
}

func TestNewPublisher_DeclareError(t *testing.T) {
	ch := &fakeChannel{declareErr: errors.New("boom")}
	if _, err := newPublisherWithChannel(ch, "gah.events"); err == nil {
		t.Fatal("expected error when ExchangeDeclare fails")
	}
}

func TestPublish_RoutingKeyIsEventType(t *testing.T) {
	ch := &fakeChannel{}
	pub := newTestPublisher(t, ch)

	err := pub.Publish(context.Background(), port.Event{
		Type:    "metric.recorded",
		Payload: map[string]any{"id": "m1"},
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if ch.lastKey != "metric.recorded" {
		t.Errorf("routing key = %q, want metric.recorded", ch.lastKey)
	}
	if ch.lastExchange != "gah.events" {
		t.Errorf("exchange = %q, want gah.events", ch.lastExchange)
	}
}

func TestPublish_BodyIsJSONOfPayload(t *testing.T) {
	ch := &fakeChannel{}
	pub := newTestPublisher(t, ch)

	payload := map[string]any{"id": "m1", "value": 42.0}
	if err := pub.Publish(context.Background(), port.Event{Type: "metric.recorded", Payload: payload}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(ch.lastMsg.Body, &got); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if got["id"] != "m1" || got["value"] != 42.0 {
		t.Errorf("body = %v, want id=m1 value=42", got)
	}
}

func TestPublish_ContentTypeAndPersistence(t *testing.T) {
	ch := &fakeChannel{}
	pub := newTestPublisher(t, ch)

	if err := pub.Publish(context.Background(), port.Event{Type: "x", Payload: nil}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if ch.lastMsg.ContentType != contentTypeJSON {
		t.Errorf("content-type = %q, want %q", ch.lastMsg.ContentType, contentTypeJSON)
	}
	if ch.lastMsg.DeliveryMode != amqp.Persistent {
		t.Errorf("delivery mode = %d, want persistent(%d)", ch.lastMsg.DeliveryMode, amqp.Persistent)
	}
}

func TestPublish_PropagatesChannelError(t *testing.T) {
	ch := &fakeChannel{publishErr: errors.New("channel closed")}
	pub := newTestPublisher(t, ch)

	if err := pub.Publish(context.Background(), port.Event{Type: "x"}); err == nil {
		t.Fatal("expected error when channel publish fails")
	}
}

func TestPublish_UnmarshalablePayload(t *testing.T) {
	ch := &fakeChannel{}
	pub := newTestPublisher(t, ch)

	// channels não são serializáveis em JSON -> erro de marshal.
	if err := pub.Publish(context.Background(), port.Event{Type: "x", Payload: make(chan int)}); err == nil {
		t.Fatal("expected marshal error for non-serializable payload")
	}
}

func TestClose_ClosesChannel(t *testing.T) {
	ch := &fakeChannel{}
	pub := newTestPublisher(t, ch)

	if err := pub.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !ch.closed {
		t.Error("expected channel to be closed")
	}
}
