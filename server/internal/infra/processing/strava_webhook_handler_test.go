package processing

import (
	"context"
	"errors"
	"testing"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/handler"
)

// fakeResolver resolve athlete->user em memória.
type fakeResolver struct {
	users  map[string]string // chave: provider:athleteID
	err    error
	called int
}

func (r *fakeResolver) FindUserByAthlete(_ context.Context, provider, athleteID string) (string, bool, error) {
	r.called++
	if r.err != nil {
		return "", false, r.err
	}
	id, ok := r.users[provider+":"+athleteID]
	return id, ok, nil
}

// fakeSyncer captura a entrada e devolve um erro controlado.
type fakeSyncer struct {
	called int
	got    usecase.SyncProviderActivitiesInput
	err    error
}

func (s *fakeSyncer) Execute(_ context.Context, in usecase.SyncProviderActivitiesInput) (*usecase.SyncProviderActivitiesOutput, error) {
	s.called++
	s.got = in
	if s.err != nil {
		return nil, s.err
	}
	return &usecase.SyncProviderActivitiesOutput{Count: 1}, nil
}

func TestStravaWebhookHandler_EventType(t *testing.T) {
	h := NewStravaWebhookHandler(&fakeResolver{}, &fakeSyncer{})
	if h.EventType() != handler.StravaWebhookEventType {
		t.Errorf("event type = %q, want %q", h.EventType(), handler.StravaWebhookEventType)
	}
}

// activityEvent simula o payload no lado do consumidor (map[string]any, ids
// numéricos como float64), tal como o subscriber entrega após json.Unmarshal.
func activityEvent(aspect string, ownerID float64) port.Event {
	return port.Event{
		Type: handler.StravaWebhookEventType,
		Payload: map[string]any{
			"provider":    "strava",
			"object_type": "activity",
			"aspect_type": aspect,
			"object_id":   float64(98765),
			"owner_id":    ownerID,
			"event_time":  float64(1700000000),
		},
	}
}

func TestStravaWebhookHandler_ActivityCreate_TriggersSync(t *testing.T) {
	resolver := &fakeResolver{users: map[string]string{"strava:12345": "user-1"}}
	syncer := &fakeSyncer{}
	h := NewStravaWebhookHandler(resolver, syncer)

	if err := h.Handle(context.Background(), activityEvent("create", 12345)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if syncer.called != 1 {
		t.Fatalf("syncer called %d times, want 1", syncer.called)
	}
	if syncer.got.UserID != "user-1" || syncer.got.Provider != "strava" {
		t.Errorf("sync input = %+v, want user-1/strava", syncer.got)
	}
}

func TestStravaWebhookHandler_NonActivity_NoOp(t *testing.T) {
	resolver := &fakeResolver{users: map[string]string{"strava:12345": "user-1"}}
	syncer := &fakeSyncer{}
	h := NewStravaWebhookHandler(resolver, syncer)

	event := port.Event{
		Type: handler.StravaWebhookEventType,
		Payload: map[string]any{
			"provider":    "strava",
			"object_type": "athlete",
			"aspect_type": "update",
			"owner_id":    float64(12345),
		},
	}
	if err := h.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if syncer.called != 0 {
		t.Errorf("syncer should not be called for non-activity, got %d", syncer.called)
	}
	if resolver.called != 0 {
		t.Errorf("resolver should not be called for non-activity, got %d", resolver.called)
	}
}

func TestStravaWebhookHandler_OtherAspect_NoOp(t *testing.T) {
	resolver := &fakeResolver{users: map[string]string{"strava:12345": "user-1"}}
	syncer := &fakeSyncer{}
	h := NewStravaWebhookHandler(resolver, syncer)

	if err := h.Handle(context.Background(), activityEvent("delete", 12345)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if syncer.called != 0 {
		t.Errorf("syncer should not be called for aspect=delete, got %d", syncer.called)
	}
}

func TestStravaWebhookHandler_AthleteNotFound_NoOpSuccess(t *testing.T) {
	resolver := &fakeResolver{users: map[string]string{}} // sem vínculo
	syncer := &fakeSyncer{}
	h := NewStravaWebhookHandler(resolver, syncer)

	// Sem erro -> ack -> sem redelivery infinita.
	if err := h.Handle(context.Background(), activityEvent("create", 99999)); err != nil {
		t.Fatalf("Handle should ack unknown athlete (no error), got: %v", err)
	}
	if syncer.called != 0 {
		t.Errorf("syncer should not be called for unknown athlete, got %d", syncer.called)
	}
}

func TestStravaWebhookHandler_SyncError_Propagates(t *testing.T) {
	resolver := &fakeResolver{users: map[string]string{"strava:12345": "user-1"}}
	syncer := &fakeSyncer{err: errors.New("boom")}
	h := NewStravaWebhookHandler(resolver, syncer)

	if err := h.Handle(context.Background(), activityEvent("create", 12345)); err == nil {
		t.Fatal("expected sync error to propagate (so message nacks -> dead-letter)")
	}
}

func TestStravaWebhookHandler_ResolverError_Propagates(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("db down")}
	syncer := &fakeSyncer{}
	h := NewStravaWebhookHandler(resolver, syncer)

	if err := h.Handle(context.Background(), activityEvent("update", 12345)); err == nil {
		t.Fatal("expected resolver error to propagate (real failure -> nack)")
	}
	if syncer.called != 0 {
		t.Errorf("syncer should not be called when resolver errors, got %d", syncer.called)
	}
}
