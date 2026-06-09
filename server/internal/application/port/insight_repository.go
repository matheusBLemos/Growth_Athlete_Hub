package port

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

type InsightRepository interface {
	Save(ctx context.Context, insight *entity.Insight) error
	SaveAll(ctx context.Context, insights []*entity.Insight) error
	FindByUserID(ctx context.Context, userID string, from, to time.Time) ([]*entity.Insight, error)
}
