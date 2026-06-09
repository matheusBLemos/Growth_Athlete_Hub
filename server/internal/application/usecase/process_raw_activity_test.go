package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

// stubRegisterActivity captura a entrada e devolve um resultado/erro controlado.
type stubRegisterActivity struct {
	called int
	gotIn  usecase.RegisterActivityInput
	out    *usecase.RegisterActivityOutput
	err    error
}

func (s *stubRegisterActivity) Execute(_ context.Context, in usecase.RegisterActivityInput) (*usecase.RegisterActivityOutput, error) {
	s.called++
	s.gotIn = in
	return s.out, s.err
}

// stubGenerateInsights devolve insights/erro controlados.
type stubGenerateInsights struct {
	called int
	gotID  string
	out    *usecase.GenerateInsightsOutput
	err    error
}

func (s *stubGenerateInsights) Execute(_ context.Context, in usecase.GenerateInsightsInput) (*usecase.GenerateInsightsOutput, error) {
	s.called++
	s.gotID = in.UserID
	return s.out, s.err
}

// stubAggregate captura a chamada de agregação.
type stubAggregate struct {
	called int
	err    error
}

func (s *stubAggregate) Execute(_ context.Context, _ usecase.AggregateDailyMetricsInput) (*usecase.AggregateDailyMetricsOutput, error) {
	s.called++
	return &usecase.AggregateDailyMetricsOutput{}, s.err
}

func validRaw() usecase.RawActivityImported {
	return usecase.RawActivityImported{
		UserID:         "u1",
		Provider:       "strava",
		ExternalID:     "ext-1",
		Type:           "Run",
		StartTime:      time.Now().Add(-2 * time.Hour),
		DurationNs:     (45 * time.Minute).Nanoseconds(),
		AvgHeartRate:   140,
		DistanceMeters: 8000,
		Name:           "Morning Run",
	}
}

func newProcessor(reg *stubRegisterActivity, gen *stubGenerateInsights, agg *stubAggregate, pub *mockEventPublisher) *usecase.ProcessRawActivity {
	return usecase.NewProcessRawActivity(reg, gen, agg, pub)
}

func TestProcessRawActivity_HappyPath_PersistsAggregatesAndPublishesInsights(t *testing.T) {
	reg := &stubRegisterActivity{out: &usecase.RegisterActivityOutput{ID: "act-1"}}
	insight, _ := entity.NewInsight("u1", valueobject.InsightTypeOvertraining, valueobject.SeverityWarning, "ease off", time.Now())
	gen := &stubGenerateInsights{out: &usecase.GenerateInsightsOutput{Count: 1, Insights: []*entity.Insight{insight}}}
	agg := &stubAggregate{}
	pub := &mockEventPublisher{}

	uc := newProcessor(reg, gen, agg, pub)
	if err := uc.Execute(context.Background(), validRaw()); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if reg.called != 1 {
		t.Errorf("RegisterActivity called %d times, want 1", reg.called)
	}
	// Normalização: "Run" -> running, duration_ns -> 45m.
	if reg.gotIn.ActivityType != string(valueobject.ActivityTypeRunning) {
		t.Errorf("activity type = %q, want running", reg.gotIn.ActivityType)
	}
	if reg.gotIn.Duration != 45*time.Minute {
		t.Errorf("duration = %v, want 45m", reg.gotIn.Duration)
	}
	if reg.gotIn.ExternalID != "ext-1" {
		t.Errorf("external id = %q, want ext-1", reg.gotIn.ExternalID)
	}
	if agg.called != 1 {
		t.Errorf("Aggregate called %d times, want 1", agg.called)
	}
	if gen.called != 1 {
		t.Errorf("GenerateInsights called %d times, want 1", gen.called)
	}

	// Publicou um insight.generated.
	if len(pub.events) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.events))
	}
	ev := pub.events[0]
	if ev.Type != usecase.InsightGeneratedEventType {
		t.Errorf("event type = %q, want %q", ev.Type, usecase.InsightGeneratedEventType)
	}
	payload, ok := ev.Payload.(usecase.InsightGenerated)
	if !ok {
		t.Fatalf("payload type = %T, want usecase.InsightGenerated", ev.Payload)
	}
	if payload.UserID != "u1" || payload.InsightID != insight.ID {
		t.Errorf("payload = %+v, want user_id=u1 insight_id=%s", payload, insight.ID)
	}
	if payload.Type != string(valueobject.InsightTypeOvertraining) || payload.Severity != string(valueobject.SeverityWarning) {
		t.Errorf("payload type/severity = %q/%q", payload.Type, payload.Severity)
	}
}

func TestProcessRawActivity_Duplicate_IsIdempotentSuccess(t *testing.T) {
	reg := &stubRegisterActivity{err: usecase.ErrDuplicateActivity}
	gen := &stubGenerateInsights{out: &usecase.GenerateInsightsOutput{}}
	agg := &stubAggregate{}
	pub := &mockEventPublisher{}

	uc := newProcessor(reg, gen, agg, pub)
	err := uc.Execute(context.Background(), validRaw())
	if err != nil {
		t.Fatalf("duplicate must be treated as success, got: %v", err)
	}
	// Duplicado: não deve gerar insights nem agregar de novo (já processado).
	if gen.called != 0 {
		t.Errorf("GenerateInsights called %d times on duplicate, want 0", gen.called)
	}
	if agg.called != 0 {
		t.Errorf("Aggregate called %d times on duplicate, want 0", agg.called)
	}
	if len(pub.events) != 0 {
		t.Errorf("published %d events on duplicate, want 0", len(pub.events))
	}
}

func TestProcessRawActivity_ValidationFailure(t *testing.T) {
	cases := map[string]func(r *usecase.RawActivityImported){
		"empty user":     func(r *usecase.RawActivityImported) { r.UserID = "" },
		"empty external": func(r *usecase.RawActivityImported) { r.ExternalID = "" },
		"zero duration":  func(r *usecase.RawActivityImported) { r.DurationNs = 0 },
		"zero starttime": func(r *usecase.RawActivityImported) { r.StartTime = time.Time{} },
		"future start":   func(r *usecase.RawActivityImported) { r.StartTime = time.Now().Add(48 * time.Hour) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			reg := &stubRegisterActivity{}
			gen := &stubGenerateInsights{}
			agg := &stubAggregate{}
			pub := &mockEventPublisher{}

			raw := validRaw()
			mutate(&raw)

			uc := newProcessor(reg, gen, agg, pub)
			err := uc.Execute(context.Background(), raw)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !errors.Is(err, usecase.ErrInvalidRawActivity) {
				t.Errorf("error = %v, want ErrInvalidRawActivity", err)
			}
			if reg.called != 0 {
				t.Error("RegisterActivity should not be called on validation failure")
			}
		})
	}
}

func TestProcessRawActivity_PersistError_Propagates(t *testing.T) {
	reg := &stubRegisterActivity{err: errors.New("db down")}
	gen := &stubGenerateInsights{}
	agg := &stubAggregate{}
	pub := &mockEventPublisher{}

	uc := newProcessor(reg, gen, agg, pub)
	err := uc.Execute(context.Background(), validRaw())
	if err == nil {
		t.Fatal("expected persist error to propagate (so message nacks)")
	}
	if gen.called != 0 {
		t.Error("GenerateInsights should not run when persist fails")
	}
}

func TestProcessRawActivity_InsightError_Propagates(t *testing.T) {
	reg := &stubRegisterActivity{out: &usecase.RegisterActivityOutput{ID: "act-1"}}
	gen := &stubGenerateInsights{err: errors.New("evaluator boom")}
	agg := &stubAggregate{}
	pub := &mockEventPublisher{}

	uc := newProcessor(reg, gen, agg, pub)
	err := uc.Execute(context.Background(), validRaw())
	if err == nil {
		t.Fatal("expected insight error to propagate (so message nacks)")
	}
}

func TestProcessRawActivity_UnknownType_MapsToOther(t *testing.T) {
	reg := &stubRegisterActivity{out: &usecase.RegisterActivityOutput{ID: "act-1"}}
	gen := &stubGenerateInsights{out: &usecase.GenerateInsightsOutput{}}
	agg := &stubAggregate{}
	pub := &mockEventPublisher{}

	raw := validRaw()
	raw.Type = "KitesurfingExtreme"

	uc := newProcessor(reg, gen, agg, pub)
	if err := uc.Execute(context.Background(), raw); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if reg.gotIn.ActivityType != string(valueobject.ActivityTypeOther) {
		t.Errorf("unknown type mapped to %q, want other", reg.gotIn.ActivityType)
	}
}
