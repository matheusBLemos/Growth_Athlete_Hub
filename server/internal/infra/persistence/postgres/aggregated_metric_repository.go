package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

var _ port.AggregatedMetricRepository = (*AggregatedMetricRepository)(nil)

// AggregatedMetricRepository persiste agregados diários de métricas na tabela
// daily_metric_aggregates. O Upsert usa ON CONFLICT na PK (user_id, date,
// metric_type) para tornar o reprocessamento de um dia idempotente.
type AggregatedMetricRepository struct {
	db *sql.DB
}

func NewAggregatedMetricRepository(db *sql.DB) *AggregatedMetricRepository {
	return &AggregatedMetricRepository{db: db}
}

func (r *AggregatedMetricRepository) Upsert(ctx context.Context, agg *port.DailyMetricAggregate) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO daily_metric_aggregates (user_id, date, metric_type, count, sum, avg, min, max, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		 ON CONFLICT (user_id, date, metric_type) DO UPDATE SET
		     count = EXCLUDED.count,
		     sum = EXCLUDED.sum,
		     avg = EXCLUDED.avg,
		     min = EXCLUDED.min,
		     max = EXCLUDED.max,
		     updated_at = NOW()`,
		agg.UserID, agg.Date, agg.MetricType, agg.Count, agg.Sum, agg.Avg, agg.Min, agg.Max,
	)
	return err
}

func (r *AggregatedMetricRepository) Find(ctx context.Context, userID string, date time.Time, metricType string) (*port.DailyMetricAggregate, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT user_id, date, metric_type, count, sum, avg, min, max
		 FROM daily_metric_aggregates
		 WHERE user_id = $1 AND date = $2 AND metric_type = $3`,
		userID, date, metricType,
	)
	var a port.DailyMetricAggregate
	err := row.Scan(&a.UserID, &a.Date, &a.MetricType, &a.Count, &a.Sum, &a.Avg, &a.Min, &a.Max)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
