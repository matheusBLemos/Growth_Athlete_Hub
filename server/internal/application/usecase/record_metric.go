package usecase

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type RecordMetricInput struct {
	UserID     string
	MetricType string
	Value      float64
	Date       time.Time
}

type RecordMetricOutput struct {
	ID string
}

type RecordMetric struct {
	metricRepo port.MetricRepository
	publisher  port.EventPublisher
}

func NewRecordMetric(repo port.MetricRepository, pub port.EventPublisher) *RecordMetric {
	return &RecordMetric{
		metricRepo: repo,
		publisher:  pub,
	}
}

func (uc *RecordMetric) Execute(ctx context.Context, input RecordMetricInput) (*RecordMetricOutput, error) {
	mt, err := valueobject.NewMetricType(input.MetricType)
	if err != nil {
		return nil, err
	}

	metric, err := entity.NewMetric(input.UserID, mt, input.Value, input.Date)
	if err != nil {
		return nil, err
	}

	if err := uc.metricRepo.Save(ctx, metric); err != nil {
		return nil, err
	}

	_ = uc.publisher.Publish(ctx, port.Event{
		Type:    "metric.recorded",
		Payload: metric,
	})

	return &RecordMetricOutput{ID: metric.ID}, nil
}
