package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
)

// stubNotifier captura o insight decodificado e devolve um erro controlado.
type stubNotifier struct {
	called int
	got    usecase.InsightGenerated
	err    error
}

func (s *stubNotifier) Execute(_ context.Context, insight usecase.InsightGenerated) error {
	s.called++
	s.got = insight
	return s.err
}

func TestInsightNotificationHandler_EventType(t *testing.T) {
	h := NewInsightNotificationHandler(&stubNotifier{})
	if h.EventType() != usecase.InsightGeneratedEventType {
		t.Errorf("event type = %q, want %q", h.EventType(), usecase.InsightGeneratedEventType)
	}
}

// TestInsightNotificationHandler_DecodesMapPayload simula o lado do consumidor,
// onde Event.Payload chega como map[string]any (JSON decodificado).
func TestInsightNotificationHandler_DecodesMapPayload(t *testing.T) {
	stub := &stubNotifier{}
	h := NewInsightNotificationHandler(stub)

	date := time.Date(2026, 6, 9, 8, 0, 0, 0, time.UTC)
	event := port.Event{
		Type: usecase.InsightGeneratedEventType,
		Payload: map[string]any{
			"user_id":    "u1",
			"insight_id": "ins-1",
			"type":       "recovery",
			"severity":   "warning",
			"message":    "Recuperação baixa.",
			"date":       date.Format(time.RFC3339),
		},
	}

	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if stub.called != 1 {
		t.Fatalf("notifier called %d times, want 1", stub.called)
	}
	if stub.got.UserID != "u1" || stub.got.InsightID != "ins-1" {
		t.Errorf("decoded = %+v, want user_id=u1 insight_id=ins-1", stub.got)
	}
	if stub.got.Type != "recovery" || stub.got.Severity != "warning" {
		t.Errorf("type/severity = %q/%q", stub.got.Type, stub.got.Severity)
	}
	if !stub.got.Date.Equal(date) {
		t.Errorf("date = %v, want %v", stub.got.Date, date)
	}
}

func TestInsightNotificationHandler_DecodesTypedPayload(t *testing.T) {
	stub := &stubNotifier{}
	h := NewInsightNotificationHandler(stub)

	insight := usecase.InsightGenerated{UserID: "u9", InsightID: "x", Type: "sleep"}
	if err := h.Handle(context.Background(), port.Event{Type: usecase.InsightGeneratedEventType, Payload: insight}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if stub.got.UserID != "u9" || stub.got.Type != "sleep" {
		t.Errorf("decoded = %+v, want user_id=u9 type=sleep", stub.got)
	}
}

func TestInsightNotificationHandler_PropagatesError(t *testing.T) {
	stub := &stubNotifier{err: errors.New("boom")}
	h := NewInsightNotificationHandler(stub)

	event := port.Event{Type: usecase.InsightGeneratedEventType, Payload: map[string]any{"user_id": "u1"}}
	if err := h.Handle(context.Background(), event); err == nil {
		t.Fatal("expected notifier error to propagate (so message nacks)")
	}
}

func TestInsightNotificationHandler_BadPayload_Errors(t *testing.T) {
	stub := &stubNotifier{}
	h := NewInsightNotificationHandler(stub)

	event := port.Event{Type: usecase.InsightGeneratedEventType, Payload: make(chan int)}
	if err := h.Handle(context.Background(), event); err == nil {
		t.Fatal("expected decode error for non-encodable payload")
	}
	if stub.called != 0 {
		t.Error("notifier should not be called on decode failure")
	}
}
