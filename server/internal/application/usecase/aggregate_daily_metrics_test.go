package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

// mockAggRepo é um repositório de agregados em memória chaveado por
// "userID|date|metricType".
type mockAggRepo struct {
	store      map[string]*port.DailyMetricAggregate
	upsertErr  error
	upsertCall int
}

func newMockAggRepo() *mockAggRepo {
	return &mockAggRepo{store: make(map[string]*port.DailyMetricAggregate)}
}

func aggKey(userID string, date time.Time, mt string) string {
	return userID + "|" + date.UTC().Format("2006-01-02") + "|" + mt
}

func (m *mockAggRepo) Upsert(_ context.Context, agg *port.DailyMetricAggregate) error {
	m.upsertCall++
	if m.upsertErr != nil {
		return m.upsertErr
	}
	cp := *agg
	m.store[aggKey(agg.UserID, agg.Date, agg.MetricType)] = &cp
	return nil
}

func (m *mockAggRepo) Find(_ context.Context, userID string, date time.Time, mt string) (*port.DailyMetricAggregate, error) {
	return m.store[aggKey(userID, date, mt)], nil
}

var _ port.AggregatedMetricRepository = (*mockAggRepo)(nil)

func TestAggregateDailyMetrics_ComputesAndUpserts(t *testing.T) {
	metricRepo := newMockMetricRepo()
	aggRepo := newMockAggRepo()

	day := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	// Três métricas de training_load no mesmo dia.
	for _, v := range []float64{100, 200, 300} {
		mt, _ := entity.NewMetric("u1", valueobject.MetricTypeTrainingLoad, v, day.Add(8*time.Hour))
		metricRepo.metrics = append(metricRepo.metrics, mt)
	}

	uc := usecase.NewAggregateDailyMetrics(metricRepo, aggRepo)
	out, err := uc.Execute(context.Background(), usecase.AggregateDailyMetricsInput{
		UserID: "u1",
		Day:    day.Add(15 * time.Hour), // qualquer instante do dia
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Count == 0 {
		t.Fatal("expected at least one aggregate written")
	}

	got, _ := aggRepo.Find(context.Background(), "u1", day, string(valueobject.MetricTypeTrainingLoad))
	if got == nil {
		t.Fatal("training_load aggregate not persisted")
	}
	if got.Count != 3 {
		t.Errorf("count = %d, want 3", got.Count)
	}
	if got.Sum != 600 {
		t.Errorf("sum = %v, want 600", got.Sum)
	}
	if got.Avg != 200 {
		t.Errorf("avg = %v, want 200", got.Avg)
	}
	if got.Min != 100 || got.Max != 300 {
		t.Errorf("min/max = %v/%v, want 100/300", got.Min, got.Max)
	}
}

func TestAggregateDailyMetrics_NoMetrics_NoOp(t *testing.T) {
	metricRepo := newMockMetricRepo()
	aggRepo := newMockAggRepo()

	uc := usecase.NewAggregateDailyMetrics(metricRepo, aggRepo)
	out, err := uc.Execute(context.Background(), usecase.AggregateDailyMetricsInput{
		UserID: "u1",
		Day:    time.Now(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Count != 0 {
		t.Errorf("count = %d, want 0 (no metrics)", out.Count)
	}
	if aggRepo.upsertCall != 0 {
		t.Errorf("upsert called %d times, want 0", aggRepo.upsertCall)
	}
}

func TestAggregateDailyMetrics_PropagatesUpsertError(t *testing.T) {
	metricRepo := newMockMetricRepo()
	aggRepo := newMockAggRepo()
	aggRepo.upsertErr = errors.New("db down")

	day := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	mt, _ := entity.NewMetric("u1", valueobject.MetricTypeTrainingLoad, 100, day.Add(8*time.Hour))
	metricRepo.metrics = append(metricRepo.metrics, mt)

	uc := usecase.NewAggregateDailyMetrics(metricRepo, aggRepo)
	_, err := uc.Execute(context.Background(), usecase.AggregateDailyMetricsInput{UserID: "u1", Day: day})
	if err == nil {
		t.Fatal("expected error propagated from upsert")
	}
}

func TestAggregateDailyMetrics_EmptyUserID(t *testing.T) {
	uc := usecase.NewAggregateDailyMetrics(newMockMetricRepo(), newMockAggRepo())
	_, err := uc.Execute(context.Background(), usecase.AggregateDailyMetricsInput{Day: time.Now()})
	if err == nil {
		t.Fatal("expected error for empty user id")
	}
}
