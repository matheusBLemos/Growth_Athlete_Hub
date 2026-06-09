package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

var _ port.MetricRepository = (*MetricRepository)(nil)

type MetricRepository struct {
	db *sql.DB
}

func NewMetricRepository(db *sql.DB) *MetricRepository {
	return &MetricRepository{db: db}
}

func (r *MetricRepository) Save(ctx context.Context, m *entity.Metric) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO metrics (id, user_id, type, value, date, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		m.ID, m.UserID, string(m.Type), m.Value, m.Date, m.CreatedAt,
	)
	return err
}

func (r *MetricRepository) FindByUserIDAndType(ctx context.Context, userID string, metricType valueobject.MetricType, from, to time.Time) ([]*entity.Metric, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, type, value, date, created_at
		 FROM metrics WHERE user_id = $1 AND type = $2 AND date >= $3 AND date <= $4
		 ORDER BY date ASC
		 LIMIT 1000`,
		userID, string(metricType), from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMetrics(rows)
}

func (r *MetricRepository) FindLatestByUserIDAndType(ctx context.Context, userID string, metricType valueobject.MetricType, limit int) ([]*entity.Metric, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, type, value, date, created_at
		 FROM metrics WHERE user_id = $1 AND type = $2
		 ORDER BY date DESC LIMIT $3`,
		userID, string(metricType), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMetrics(rows)
}

func scanMetrics(rows *sql.Rows) ([]*entity.Metric, error) {
	var metrics []*entity.Metric
	for rows.Next() {
		var m entity.Metric
		var mt string
		err := rows.Scan(&m.ID, &m.UserID, &mt, &m.Value, &m.Date, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		m.Type = valueobject.MetricType(mt)
		metrics = append(metrics, &m)
	}
	return metrics, rows.Err()
}
