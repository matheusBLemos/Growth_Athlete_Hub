package deterministic

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

const restingHRBaselineDays = 7
const restingHRRiseThreshold = 0.10

type RestingHRRule struct{}

func NewRestingHRRule() *RestingHRRule {
	return &RestingHRRule{}
}

func (r *RestingHRRule) Evaluate(_ context.Context, userID string, metrics []*entity.Metric) ([]*entity.Insight, error) {
	hrMetrics := filterByType(metrics, valueobject.MetricTypeRestingHR)
	if len(hrMetrics) < restingHRBaselineDays+1 {
		return nil, nil
	}

	sort.Slice(hrMetrics, func(i, j int) bool {
		return hrMetrics[i].Date.Before(hrMetrics[j].Date)
	})

	current := hrMetrics[len(hrMetrics)-1]
	baselineSlice := hrMetrics[len(hrMetrics)-1-restingHRBaselineDays : len(hrMetrics)-1]

	avg := average(baselineSlice)
	if avg == 0 {
		return nil, nil
	}

	rise := (current.Value - avg) / avg
	if rise >= restingHRRiseThreshold {
		insight, err := entity.NewInsight(
			userID,
			valueobject.InsightTypeRestingHRHigh,
			valueobject.SeverityWarning,
			fmt.Sprintf("Resting heart rate rose %.0f%% above 7-day baseline (current: %.0f, baseline: %.0f)", rise*100, current.Value, avg),
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		return []*entity.Insight{insight}, nil
	}

	return nil, nil
}
