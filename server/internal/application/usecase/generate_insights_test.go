package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

func TestGenerateInsights_Success(t *testing.T) {
	metricRepo := newMockMetricRepo()
	insightRepo := newMockInsightRepo()

	hrvInsight, _ := entity.NewInsight("user-1", valueobject.InsightTypeHRVDrop, valueobject.SeverityWarning, "HRV dropped 20%", time.Now())
	evaluator := &mockInsightEvaluator{
		result: []*entity.Insight{hrvInsight},
	}

	for i := 0; i < 7; i++ {
		m, _ := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 65, time.Now().AddDate(0, 0, -i))
		metricRepo.metrics = append(metricRepo.metrics, m)
	}

	uc := usecase.NewGenerateInsights(metricRepo, insightRepo, evaluator)

	output, err := uc.Execute(context.Background(), usecase.GenerateInsightsInput{UserID: "user-1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.Count != 1 {
		t.Fatalf("expected 1 insight, got %d", output.Count)
	}
	if insightRepo.saveCalled != 1 {
		t.Fatalf("expected save called once, got %d", insightRepo.saveCalled)
	}
}

func TestGenerateInsights_NoMetrics(t *testing.T) {
	metricRepo := newMockMetricRepo()
	insightRepo := newMockInsightRepo()
	evaluator := &mockInsightEvaluator{result: nil}

	uc := usecase.NewGenerateInsights(metricRepo, insightRepo, evaluator)

	output, err := uc.Execute(context.Background(), usecase.GenerateInsightsInput{UserID: "user-1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.Count != 0 {
		t.Fatalf("expected 0 insights, got %d", output.Count)
	}
}

func TestGenerateInsights_EmptyUserID(t *testing.T) {
	metricRepo := newMockMetricRepo()
	insightRepo := newMockInsightRepo()
	evaluator := &mockInsightEvaluator{}

	uc := usecase.NewGenerateInsights(metricRepo, insightRepo, evaluator)

	_, err := uc.Execute(context.Background(), usecase.GenerateInsightsInput{UserID: ""})
	if err != entity.ErrEmptyUserID {
		t.Fatalf("expected ErrEmptyUserID, got %v", err)
	}
}
