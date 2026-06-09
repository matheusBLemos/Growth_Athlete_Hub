package usecase

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type GenerateInsightsInput struct {
	UserID string
}

type GenerateInsightsOutput struct {
	Count    int
	Insights []*entity.Insight
}

type GenerateInsights struct {
	metricRepo  port.MetricRepository
	insightRepo port.InsightRepository
	evaluator   port.InsightEvaluator
}

func NewGenerateInsights(metricRepo port.MetricRepository, insightRepo port.InsightRepository, evaluator port.InsightEvaluator) *GenerateInsights {
	return &GenerateInsights{
		metricRepo:  metricRepo,
		insightRepo: insightRepo,
		evaluator:   evaluator,
	}
}

func (uc *GenerateInsights) Execute(ctx context.Context, input GenerateInsightsInput) (*GenerateInsightsOutput, error) {
	if input.UserID == "" {
		return nil, entity.ErrEmptyUserID
	}

	now := time.Now()
	from := now.AddDate(0, 0, -30)

	metricTypes := []valueobject.MetricType{
		valueobject.MetricTypeHRV,
		valueobject.MetricTypeRestingHR,
		valueobject.MetricTypeSleepDuration,
		valueobject.MetricTypeTrainingLoad,
	}

	var allMetrics []*entity.Metric
	for _, mt := range metricTypes {
		metrics, err := uc.metricRepo.FindByUserIDAndType(ctx, input.UserID, mt, from, now)
		if err != nil {
			return nil, err
		}
		allMetrics = append(allMetrics, metrics...)
	}

	insights, err := uc.evaluator.Evaluate(ctx, input.UserID, allMetrics)
	if err != nil {
		return nil, err
	}

	if len(insights) > 0 {
		if err := uc.insightRepo.SaveAll(ctx, insights); err != nil {
			return nil, err
		}
	}

	return &GenerateInsightsOutput{
		Count:    len(insights),
		Insights: insights,
	}, nil
}
