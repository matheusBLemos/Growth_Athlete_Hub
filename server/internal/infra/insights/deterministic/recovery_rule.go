package deterministic

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

const recoverySignalThreshold = 2

type RecoveryRule struct{}

func NewRecoveryRule() *RecoveryRule {
	return &RecoveryRule{}
}

func (r *RecoveryRule) Evaluate(ctx context.Context, userID string, metrics []*entity.Metric) ([]*entity.Insight, error) {
	signals := 0

	hrvRule := NewHRVRule()
	hrvInsights, err := hrvRule.Evaluate(ctx, userID, metrics)
	if err != nil {
		return nil, err
	}
	if len(hrvInsights) > 0 {
		signals++
	}

	sleepRule := NewSleepRule()
	sleepInsights, err := sleepRule.Evaluate(ctx, userID, metrics)
	if err != nil {
		return nil, err
	}
	if len(sleepInsights) > 0 {
		signals++
	}

	restingHRRule := NewRestingHRRule()
	restingHRInsights, err := restingHRRule.Evaluate(ctx, userID, metrics)
	if err != nil {
		return nil, err
	}
	if len(restingHRInsights) > 0 {
		signals++
	}

	if signals >= recoverySignalThreshold {
		insight, err := entity.NewInsight(
			userID,
			valueobject.InsightTypeRecoveryNeeded,
			valueobject.SeverityCritical,
			"Multiple stress signals detected — consider a recovery day",
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		return []*entity.Insight{insight}, nil
	}

	return nil, nil
}
