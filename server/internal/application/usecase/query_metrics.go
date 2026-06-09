package usecase

import (
	"context"
	"encoding/json"
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
	cache      port.Cache
	cacheTTL   time.Duration
}

// NewQueryMetrics constrói a use case de leitura de métricas com cache-aside.
// cache pode ser nil — nesse caso a use case opera sem cache. cacheTTL é o TTL
// das entradas de query no cache.
func NewQueryMetrics(repo port.MetricRepository, cache port.Cache, cacheTTL time.Duration) *QueryMetrics {
	return &QueryMetrics{
		metricRepo: repo,
		cache:      cache,
		cacheTTL:   cacheTTL,
	}
}

func (uc *QueryMetrics) Execute(ctx context.Context, input QueryMetricsInput) (*QueryMetricsOutput, error) {
	if input.UserID == "" {
		return nil, entity.ErrEmptyUserID
	}

	mt, err := valueobject.NewMetricType(input.MetricType)
	if err != nil {
		return nil, err
	}

	// Cache-aside: tenta servir do cache. Qualquer erro de cache é ignorado e
	// caímos para o repositório (resiliência — cache nunca derruba a request).
	var key string
	if uc.cache != nil {
		ver := metricsVersion(ctx, uc.cache, input.UserID)
		key = metricsQueryKey(input.UserID, ver, string(mt), input.From, input.To)

		if raw, hit, gerr := uc.cache.Get(ctx, key); gerr != nil {
			port.LoggerFromContext(ctx).Warn(ctx, "query_metrics: cache get error (falling back to repo)",
				"user_id", input.UserID, "error", gerr)
		} else if hit {
			var metrics []*entity.Metric
			if uerr := json.Unmarshal(raw, &metrics); uerr == nil {
				return &QueryMetricsOutput{Metrics: metrics}, nil
			}
			port.LoggerFromContext(ctx).Warn(ctx, "query_metrics: cache unmarshal error (falling back to repo)",
				"user_id", input.UserID)
		}
	}

	metrics, err := uc.metricRepo.FindByUserIDAndType(ctx, input.UserID, mt, input.From, input.To)
	if err != nil {
		return nil, err
	}

	if uc.cache != nil {
		if raw, merr := json.Marshal(metrics); merr == nil {
			if serr := uc.cache.Set(ctx, key, raw, uc.cacheTTL); serr != nil {
				port.LoggerFromContext(ctx).Warn(ctx, "query_metrics: cache set error (ignored)",
					"user_id", input.UserID, "error", serr)
			}
		}
	}

	return &QueryMetricsOutput{Metrics: metrics}, nil
}
