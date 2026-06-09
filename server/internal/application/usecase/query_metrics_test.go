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

func TestQueryMetrics_Success(t *testing.T) {
	repo := newMockMetricRepo()

	m1, _ := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 65, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	m2, _ := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 60, time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC))
	repo.metrics = append(repo.metrics, m1, m2)

	uc := usecase.NewQueryMetrics(repo, nil, 0)

	output, err := uc.Execute(context.Background(), usecase.QueryMetricsInput{
		UserID:     "user-1",
		MetricType: "hrv",
		From:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2025, 6, 3, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(output.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(output.Metrics))
	}
}

func TestQueryMetrics_InvalidMetricType(t *testing.T) {
	repo := newMockMetricRepo()
	uc := usecase.NewQueryMetrics(repo, nil, 0)

	_, err := uc.Execute(context.Background(), usecase.QueryMetricsInput{
		UserID:     "user-1",
		MetricType: "invalid",
		From:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2025, 6, 3, 0, 0, 0, 0, time.UTC),
	})
	if err != valueobject.ErrInvalidMetricType {
		t.Fatalf("expected ErrInvalidMetricType, got %v", err)
	}
}

func TestQueryMetrics_EmptyUserID(t *testing.T) {
	repo := newMockMetricRepo()
	uc := usecase.NewQueryMetrics(repo, nil, 0)

	_, err := uc.Execute(context.Background(), usecase.QueryMetricsInput{
		UserID:     "",
		MetricType: "hrv",
		From:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2025, 6, 3, 0, 0, 0, 0, time.UTC),
	})
	if err != entity.ErrEmptyUserID {
		t.Fatalf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestQueryMetrics_FiltersByPeriod(t *testing.T) {
	repo := newMockMetricRepo()

	m1, _ := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 65, time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC))
	m2, _ := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 60, time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC))
	repo.metrics = append(repo.metrics, m1, m2)

	uc := usecase.NewQueryMetrics(repo, nil, 0)

	output, err := uc.Execute(context.Background(), usecase.QueryMetricsInput{
		UserID:     "user-1",
		MetricType: "hrv",
		From:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(output.Metrics) != 1 {
		t.Fatalf("expected 1 metric (filtered by period), got %d", len(output.Metrics))
	}
}

func queryInput() usecase.QueryMetricsInput {
	return usecase.QueryMetricsInput{
		UserID:     "user-1",
		MetricType: "hrv",
		From:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2025, 6, 3, 0, 0, 0, 0, time.UTC),
	}
}

func TestQueryMetrics_CacheMiss_StoresResult(t *testing.T) {
	repo := newMockMetricRepo()
	m1, _ := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 65, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	repo.metrics = append(repo.metrics, m1)
	cache := newFakeCache()

	uc := usecase.NewQueryMetrics(repo, cache, time.Minute)

	out, err := uc.Execute(context.Background(), queryInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(out.Metrics))
	}
	// Miss => o resultado deve ter sido escrito no cache.
	if cache.setCalls == 0 {
		t.Fatal("expected result to be stored in cache on miss")
	}
}

func TestQueryMetrics_CacheHit_RepoNotCalled(t *testing.T) {
	repo := newMockMetricRepo()
	m1, _ := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 65, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	repo.metrics = append(repo.metrics, m1)
	cache := newFakeCache()

	uc := usecase.NewQueryMetrics(repo, cache, time.Minute)

	// Primeira chamada: popula o cache.
	if _, err := uc.Execute(context.Background(), queryInput()); err != nil {
		t.Fatalf("first execute: %v", err)
	}
	repo.findCalls = 0

	// Segunda chamada: deve servir do cache, sem tocar no repo.
	out, err := uc.Execute(context.Background(), queryInput())
	if err != nil {
		t.Fatalf("second execute: %v", err)
	}
	if repo.findCalls != 0 {
		t.Fatalf("expected repo NOT called on cache hit, got %d calls", repo.findCalls)
	}
	if len(out.Metrics) != 1 {
		t.Fatalf("expected 1 metric from cache, got %d", len(out.Metrics))
	}
}

func TestQueryMetrics_CacheError_FallsBackToRepo(t *testing.T) {
	repo := newMockMetricRepo()
	m1, _ := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 65, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	repo.metrics = append(repo.metrics, m1)
	cache := newFakeCache()
	cache.getErr = errors.New("redis down")
	cache.setErr = errors.New("redis down")

	uc := usecase.NewQueryMetrics(repo, cache, time.Minute)

	out, err := uc.Execute(context.Background(), queryInput())
	if err != nil {
		t.Fatalf("cache error must not fail the request, got %v", err)
	}
	if repo.findCalls != 1 {
		t.Fatalf("expected repo called once on cache error, got %d", repo.findCalls)
	}
	if len(out.Metrics) != 1 {
		t.Fatalf("expected 1 metric from repo, got %d", len(out.Metrics))
	}
}
