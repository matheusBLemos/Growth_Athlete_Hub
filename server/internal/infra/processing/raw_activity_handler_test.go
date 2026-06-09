package processing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
)

// stubProcessor captura a entrada decodificada e devolve um erro controlado.
type stubProcessor struct {
	called int
	got    usecase.RawActivityImported
	err    error
}

func (s *stubProcessor) Execute(_ context.Context, raw usecase.RawActivityImported) error {
	s.called++
	s.got = raw
	return s.err
}

func TestRawActivityHandler_EventType(t *testing.T) {
	h := NewRawActivityHandler(&stubProcessor{})
	if h.EventType() != usecase.RawActivityImportedEventType {
		t.Errorf("event type = %q, want %q", h.EventType(), usecase.RawActivityImportedEventType)
	}
}

// TestRawActivityHandler_DecodesMapPayload simula o lado do consumidor, onde
// Event.Payload chega como JSON decodificado em map[string]any (não como struct
// tipada). O handler deve re-serializar/deserializar no tipo correto.
func TestRawActivityHandler_DecodesMapPayload(t *testing.T) {
	proc := &stubProcessor{}
	h := NewRawActivityHandler(proc)

	start := time.Date(2026, 6, 9, 7, 0, 0, 0, time.UTC)
	event := port.Event{
		Type: usecase.RawActivityImportedEventType,
		Payload: map[string]any{
			"user_id":         "u1",
			"provider":        "strava",
			"external_id":     "ext-42",
			"type":            "Run",
			"start_time":      start.Format(time.RFC3339),
			"duration_ns":     float64((30 * time.Minute).Nanoseconds()), // JSON numbers -> float64
			"avg_heart_rate":  float64(145),
			"distance_meters": 5000.0,
			"name":            "Easy Run",
		},
	}

	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if proc.called != 1 {
		t.Fatalf("processor called %d times, want 1", proc.called)
	}
	if proc.got.UserID != "u1" || proc.got.ExternalID != "ext-42" {
		t.Errorf("decoded = %+v, want user_id=u1 external_id=ext-42", proc.got)
	}
	if proc.got.Type != "Run" {
		t.Errorf("type = %q, want Run", proc.got.Type)
	}
	if proc.got.DurationNs != (30 * time.Minute).Nanoseconds() {
		t.Errorf("duration_ns = %d, want %d", proc.got.DurationNs, (30 * time.Minute).Nanoseconds())
	}
	if proc.got.AvgHeartRate != 145 {
		t.Errorf("avg_heart_rate = %d, want 145", proc.got.AvgHeartRate)
	}
	if !proc.got.StartTime.Equal(start) {
		t.Errorf("start_time = %v, want %v", proc.got.StartTime, start)
	}
}

// TestRawActivityHandler_DecodesTypedPayload garante que um payload já tipado
// (ex.: in-process) também é aceito.
func TestRawActivityHandler_DecodesTypedPayload(t *testing.T) {
	proc := &stubProcessor{}
	h := NewRawActivityHandler(proc)

	raw := usecase.RawActivityImported{UserID: "u9", ExternalID: "x", Type: "Ride", DurationNs: 100}
	if err := h.Handle(context.Background(), port.Event{Type: usecase.RawActivityImportedEventType, Payload: raw}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if proc.got.UserID != "u9" || proc.got.Type != "Ride" {
		t.Errorf("decoded = %+v, want user_id=u9 type=Ride", proc.got)
	}
}

func TestRawActivityHandler_PropagatesProcessorError(t *testing.T) {
	proc := &stubProcessor{err: errors.New("boom")}
	h := NewRawActivityHandler(proc)

	event := port.Event{Type: usecase.RawActivityImportedEventType, Payload: map[string]any{"user_id": "u1"}}
	if err := h.Handle(context.Background(), event); err == nil {
		t.Fatal("expected processor error to propagate (so message nacks)")
	}
}

func TestRawActivityHandler_BadPayload_Errors(t *testing.T) {
	proc := &stubProcessor{}
	h := NewRawActivityHandler(proc)

	// Payload não serializável para o tipo (canal não é JSON-encodable).
	event := port.Event{Type: usecase.RawActivityImportedEventType, Payload: make(chan int)}
	if err := h.Handle(context.Background(), event); err == nil {
		t.Fatal("expected decode error for non-encodable payload")
	}
	if proc.called != 0 {
		t.Error("processor should not be called on decode failure")
	}
}
