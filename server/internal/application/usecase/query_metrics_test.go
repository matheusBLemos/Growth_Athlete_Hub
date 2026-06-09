package usecase_test

import (
	"context"
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

	uc := usecase.NewQueryMetrics(repo)

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
	uc := usecase.NewQueryMetrics(repo)

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
	uc := usecase.NewQueryMetrics(repo)

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

	uc := usecase.NewQueryMetrics(repo)

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
