package deterministic

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

const (
	sleepWarningThreshold  = 6.0
	sleepCriticalThreshold = 5.0
	sleepConsecutiveDays   = 3
)

type SleepRule struct{}

func NewSleepRule() *SleepRule {
	return &SleepRule{}
}

func (r *SleepRule) Evaluate(_ context.Context, userID string, metrics []*entity.Metric) ([]*entity.Insight, error) {
	sleepMetrics := filterByType(metrics, valueobject.MetricTypeSleepDuration)
	if len(sleepMetrics) < sleepConsecutiveDays {
		return nil, nil
	}

	sort.Slice(sleepMetrics, func(i, j int) bool {
		return sleepMetrics[i].Date.Before(sleepMetrics[j].Date)
	})

	var insights []*entity.Insight

	recentCriticalCount := 0
	for i := len(sleepMetrics) - 1; i >= 0 && i >= len(sleepMetrics)-sleepConsecutiveDays; i-- {
		if sleepMetrics[i].Value < sleepCriticalThreshold {
			recentCriticalCount++
		}
	}
	if recentCriticalCount >= sleepConsecutiveDays {
		insight, err := entity.NewInsight(
			userID,
			valueobject.InsightTypeSleepDeficit,
			valueobject.SeverityCritical,
			fmt.Sprintf("Sleep duration critically low (below %.0fh) for %d consecutive days", sleepCriticalThreshold, sleepConsecutiveDays),
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		return []*entity.Insight{insight}, nil
	}

	consecutiveBelow6 := 0
	for i := len(sleepMetrics) - 1; i >= 0; i-- {
		if sleepMetrics[i].Value < sleepWarningThreshold {
			consecutiveBelow6++
		} else {
			break
		}
	}
	if consecutiveBelow6 >= sleepConsecutiveDays {
		insight, err := entity.NewInsight(
			userID,
			valueobject.InsightTypeSleepDeficit,
			valueobject.SeverityWarning,
			fmt.Sprintf("Sleep duration below %.0fh for %d consecutive days", sleepWarningThreshold, consecutiveBelow6),
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		insights = append(insights, insight)
	}

	return insights, nil
}
