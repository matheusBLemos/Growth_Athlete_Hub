package port

import (
	"context"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

type InsightEvaluator interface {
	Evaluate(ctx context.Context, userID string, metrics []*entity.Metric) ([]*entity.Insight, error)
}
