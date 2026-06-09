package port

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type MetricRepository interface {
	Save(ctx context.Context, metric *entity.Metric) error
	FindByUserIDAndType(ctx context.Context, userID string, metricType valueobject.MetricType, from, to time.Time) ([]*entity.Metric, error)
	FindLatestByUserIDAndType(ctx context.Context, userID string, metricType valueobject.MetricType, limit int) ([]*entity.Metric, error)
}
