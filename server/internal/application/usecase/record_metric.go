package usecase

import (
	"context"
	"log"
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
	cache      port.Cache
	cacheTTL   time.Duration
}

// NewRecordMetric constrói a use case de escrita de métricas. cache pode ser nil
// (opera sem invalidação). Após gravar, bumpa a versão de cache do usuário para
// invalidar as queries cacheadas (ver metrics_cache.go). cacheTTL é o TTL da
// chave de versão.
func NewRecordMetric(repo port.MetricRepository, pub port.EventPublisher, cache port.Cache, cacheTTL time.Duration) *RecordMetric {
	return &RecordMetric{
		metricRepo: repo,
		publisher:  pub,
		cache:      cache,
		cacheTTL:   cacheTTL,
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

	// Invalida as queries cacheadas do usuário bumpando sua versão de cache.
	// Erro de cache não falha a escrita (resiliência) — apenas logamos.
	if err := bumpMetricsVersion(ctx, uc.cache, metric.UserID, uc.cacheTTL); err != nil {
		log.Printf("record_metric: cache version bump error (ignored): %v", err)
	}

	if err := uc.publisher.Publish(ctx, port.Event{
		Type:    "metric.recorded",
		Payload: metric,
	}); err != nil {
		log.Printf("failed to publish metric.recorded event: %v", err)
	}

	return &RecordMetricOutput{ID: metric.ID}, nil
}
