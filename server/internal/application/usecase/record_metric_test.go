package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

func TestRecordMetric_Success(t *testing.T) {
	repo := newMockMetricRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRecordMetric(repo, pub, nil, 0)

	input := usecase.RecordMetricInput{
		UserID:     "user-1",
		MetricType: "hrv",
		Value:      65.0,
		Date:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	output, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if repo.saveCalled != 1 {
		t.Fatalf("expected save called once, got %d", repo.saveCalled)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
}

func TestRecordMetric_InvalidMetricType(t *testing.T) {
	repo := newMockMetricRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRecordMetric(repo, pub, nil, 0)

	input := usecase.RecordMetricInput{
		UserID:     "user-1",
		MetricType: "invalid",
		Value:      65.0,
		Date:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	_, err := uc.Execute(context.Background(), input)
	if err != valueobject.ErrInvalidMetricType {
		t.Fatalf("expected ErrInvalidMetricType, got %v", err)
	}
}

func TestRecordMetric_OutOfRange(t *testing.T) {
	repo := newMockMetricRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRecordMetric(repo, pub, nil, 0)

	input := usecase.RecordMetricInput{
		UserID:     "user-1",
		MetricType: "hrv",
		Value:      -10,
		Date:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	_, err := uc.Execute(context.Background(), input)
	if err != entity.ErrMetricOutOfRange {
		t.Fatalf("expected ErrMetricOutOfRange, got %v", err)
	}
}

func TestRecordMetric_InvalidatesCachedQueries(t *testing.T) {
	repo := newMockMetricRepo()
	pub := &mockEventPublisher{}
	cache := newFakeCache()

	seed, _ := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 65, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	repo.metrics = append(repo.metrics, seed)

	query := usecase.NewQueryMetrics(repo, cache, time.Minute)
	record := usecase.NewRecordMetric(repo, pub, cache, time.Minute)

	in := usecase.QueryMetricsInput{
		UserID:     "user-1",
		MetricType: "hrv",
		From:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC),
	}

	// 1) Primeira leitura popula o cache.
	first, err := query.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("first query: %v", err)
	}
	if len(first.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(first.Metrics))
	}

	// 2) Escreve uma nova métrica -> deve invalidar (bump da versão).
	_, err = record.Execute(context.Background(), usecase.RecordMetricInput{
		UserID:     "user-1",
		MetricType: "hrv",
		Value:      70,
		Date:       time.Date(2025, 6, 5, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	repo.findCalls = 0

	// 3) Leitura seguinte NÃO pode servir a entrada velha (1 métrica); precisa
	// recorrer ao repo e enxergar as 2 métricas.
	second, err := query.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("second query: %v", err)
	}
	if repo.findCalls != 1 {
		t.Fatalf("expected repo hit after invalidation, got %d find calls", repo.findCalls)
	}
	if len(second.Metrics) != 2 {
		t.Fatalf("expected 2 metrics after write+invalidation, got %d", len(second.Metrics))
	}
}
