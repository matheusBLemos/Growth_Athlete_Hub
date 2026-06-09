package usecase

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type QueryMetricsInput struct {
	UserID     string
	MetricType string
	From       time.Time
	To         time.Time
}

type QueryMetricsOutput struct {
	Metrics []*entity.Metric
}

type QueryMetrics struct {
	metricRepo port.MetricRepository
}

func NewQueryMetrics(repo port.MetricRepository) *QueryMetrics {
	return &QueryMetrics{metricRepo: repo}
}

func (uc *QueryMetrics) Execute(ctx context.Context, input QueryMetricsInput) (*QueryMetricsOutput, error) {
	if input.UserID == "" {
		return nil, entity.ErrEmptyUserID
	}

	mt, err := valueobject.NewMetricType(input.MetricType)
	if err != nil {
		return nil, err
	}

	metrics, err := uc.metricRepo.FindByUserIDAndType(ctx, input.UserID, mt, input.From, input.To)
	if err != nil {
		return nil, err
	}

	return &QueryMetricsOutput{Metrics: metrics}, nil
}
