package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// fakeDelivery é uma entrega falsa que registra ack/nack para asserção.
type fakeDelivery struct {
	body       []byte
	routingKey string

	acked       bool
	nacked      bool
	nackRequeue bool
	ackErr      error
}

func (f *fakeDelivery) Body() []byte       { return f.body }
func (f *fakeDelivery) RoutingKey() string { return f.routingKey }
func (f *fakeDelivery) Ack() error {
	f.acked = true
	return f.ackErr
}
func (f *fakeDelivery) Nack(requeue bool) error {
	f.nacked = true
	f.nackRequeue = requeue
	return nil
}

// recordingHandler captura o evento recebido e pode forçar erro.
type recordingHandler struct {
	eventType string
	got       *port.Event
	err       error
	called    bool
}

func (h *recordingHandler) EventType() string { return h.eventType }
func (h *recordingHandler) Handle(_ context.Context, e port.Event) error {
	h.called = true
	h.got = &e
	return h.err
}

func newTestSubscriber(t *testing.T) *Subscriber {
	t.Helper()
	return &Subscriber{
		exchange:    "gah.events",
		queuePrefix: "gah",
		handlers:    make(map[string]MessageHandler),
	}
}

func TestDispatch_DecodesAndCallsHandler_ThenAcks(t *testing.T) {
	s := newTestSubscriber(t)
	h := &recordingHandler{eventType: "metric.recorded"}

	body, _ := json.Marshal(port.Event{Type: "metric.recorded", Payload: map[string]any{"id": "m1"}})
	d := &fakeDelivery{body: body, routingKey: "metric.recorded"}

	s.dispatch(context.Background(), h, d)

	if !h.called {
		t.Fatal("handler was not called")
	}
	if h.got.Type != "metric.recorded" {
		t.Errorf("event type = %q, want metric.recorded", h.got.Type)
	}
	if !d.acked {
		t.Error("delivery should be acked on success")
	}
	if d.nacked {
		t.Error("delivery should not be nacked on success")
	}
}

func TestDispatch_HandlerError_NacksWithoutRequeue(t *testing.T) {
	s := newTestSubscriber(t)
	h := &recordingHandler{eventType: "metric.recorded", err: errors.New("boom")}

	body, _ := json.Marshal(port.Event{Type: "metric.recorded"})
	d := &fakeDelivery{body: body, routingKey: "metric.recorded"}

	s.dispatch(context.Background(), h, d)

	if !d.nacked {
		t.Error("delivery should be nacked on handler error")
	}
	if d.nackRequeue {
		t.Error("nack should not requeue (routes to dead-letter)")
	}
	if d.acked {
		t.Error("delivery should not be acked on error")
	}
}

func TestDispatch_MalformedJSON_NacksWithoutRequeue(t *testing.T) {
	s := newTestSubscriber(t)
	h := &recordingHandler{eventType: "metric.recorded"}

	d := &fakeDelivery{body: []byte("not json{{"), routingKey: "metric.recorded"}

	s.dispatch(context.Background(), h, d)

	if h.called {
		t.Error("handler should not be called for malformed JSON")
	}
	if !d.nacked || d.nackRequeue {
		t.Error("malformed message should be nacked without requeue")
	}
}

func TestRegister_RoutesByEventType(t *testing.T) {
	s := newTestSubscriber(t)
	h1 := &recordingHandler{eventType: "metric.recorded"}
	h2 := &recordingHandler{eventType: "activity.registered"}
	s.Register(h1)
	s.Register(h2)

	// Dispatch usando o handler resolvido pelo mapa, simulando o roteamento.
	body, _ := json.Marshal(port.Event{Type: "activity.registered"})
	d := &fakeDelivery{body: body, routingKey: "activity.registered"}
	s.dispatch(context.Background(), s.handlers["activity.registered"], d)

	if !h2.called {
		t.Error("activity.registered handler should be called")
	}
	if h1.called {
		t.Error("metric.recorded handler should not be called")
	}
}

func TestRegisterFunc_Works(t *testing.T) {
	s := newTestSubscriber(t)
	called := false
	s.RegisterFunc("x.event", func(_ context.Context, _ port.Event) error {
		called = true
		return nil
	})

	body, _ := json.Marshal(port.Event{Type: "x.event"})
	d := &fakeDelivery{body: body}
	s.dispatch(context.Background(), s.handlers["x.event"], d)

	if !called {
		t.Error("registered func handler should be called")
	}
	if !d.acked {
		t.Error("delivery should be acked")
	}
}

// fakeSubChannel verifica a montagem (exchange/qos/queue/bind/consume).
type fakeSubChannel struct {
	declaredExchanges []string
	qosPrefetch       int
	declaredQueues    []string
	binds             []string
	consumed          []string
}

func (f *fakeSubChannel) ExchangeDeclare(name, _ string, _, _, _, _ bool, _ amqp.Table) error {
	f.declaredExchanges = append(f.declaredExchanges, name)
	return nil
}
func (f *fakeSubChannel) QueueDeclare(name string, _, _, _, _ bool, _ amqp.Table) (amqp.Queue, error) {
	f.declaredQueues = append(f.declaredQueues, name)
	return amqp.Queue{Name: name}, nil
}
func (f *fakeSubChannel) QueueBind(name, key, exchange string, _ bool, _ amqp.Table) error {
	f.binds = append(f.binds, name+"->"+exchange+"["+key+"]")
	return nil
}
func (f *fakeSubChannel) Qos(prefetchCount, _ int, _ bool) error {
	f.qosPrefetch = prefetchCount
	return nil
}
func (f *fakeSubChannel) Consume(queue, _ string, _, _, _, _ bool, _ amqp.Table) (<-chan amqp.Delivery, error) {
	f.consumed = append(f.consumed, queue)
	ch := make(chan amqp.Delivery)
	close(ch)
	return ch, nil
}
func (f *fakeSubChannel) Close() error { return nil }

func TestNewSubscriberWithChannel_DeclaresExchangeAndQos(t *testing.T) {
	ch := &fakeSubChannel{}
	sub, err := newSubscriberWithChannel(ch, "gah.events", "gah", 7)
	if err != nil {
		t.Fatalf("newSubscriberWithChannel: %v", err)
	}
	if sub == nil {
		t.Fatal("nil subscriber")
	}
	if len(ch.declaredExchanges) != 1 || ch.declaredExchanges[0] != "gah.events" {
		t.Errorf("exchanges = %v, want [gah.events]", ch.declaredExchanges)
	}
	if ch.qosPrefetch != 7 {
		t.Errorf("qos prefetch = %d, want 7", ch.qosPrefetch)
	}
}

func TestStart_DeclaresQueuesAndBinds(t *testing.T) {
	ch := &fakeSubChannel{}
	sub, err := newSubscriberWithChannel(ch, "gah.events", "gah", 5)
	if err != nil {
		t.Fatalf("newSubscriberWithChannel: %v", err)
	}
	sub.RegisterFunc("metric.recorded", func(_ context.Context, _ port.Event) error { return nil })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Start deve declarar/bind e retornar logo (ctx já cancelado).

	if err := sub.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// fila principal + dlq declaradas
	wantQueue := "gah.metric.recorded"
	foundQueue, foundDLQ := false, false
	for _, q := range ch.declaredQueues {
		if q == wantQueue {
			foundQueue = true
		}
		if q == wantQueue+".dead" {
			foundDLQ = true
		}
	}
	if !foundQueue {
		t.Errorf("queues = %v, want to contain %q", ch.declaredQueues, wantQueue)
	}
	if !foundDLQ {
		t.Errorf("queues = %v, want to contain dead-letter queue", ch.declaredQueues)
	}
	if len(ch.consumed) != 1 || ch.consumed[0] != wantQueue {
		t.Errorf("consumed = %v, want [%s]", ch.consumed, wantQueue)
	}
}

func TestStart_NoHandlers_Errors(t *testing.T) {
	ch := &fakeSubChannel{}
	sub, err := newSubscriberWithChannel(ch, "gah.events", "gah", 5)
	if err != nil {
		t.Fatalf("newSubscriberWithChannel: %v", err)
	}
	if err := sub.Start(context.Background()); err == nil {
		t.Fatal("expected error when no handlers registered")
	}
}
