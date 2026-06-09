package deterministic

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

const hrvBaselineDays = 7
const hrvDropThreshold = 0.15

type HRVRule struct{}

func NewHRVRule() *HRVRule {
	return &HRVRule{}
}

func (r *HRVRule) Evaluate(_ context.Context, userID string, metrics []*entity.Metric) ([]*entity.Insight, error) {
	hrvMetrics := filterByType(metrics, valueobject.MetricTypeHRV)
	if len(hrvMetrics) < hrvBaselineDays+1 {
		return nil, nil
	}

	sort.Slice(hrvMetrics, func(i, j int) bool {
		return hrvMetrics[i].Date.Before(hrvMetrics[j].Date)
	})

	current := hrvMetrics[len(hrvMetrics)-1]
	baselineSlice := hrvMetrics[len(hrvMetrics)-1-hrvBaselineDays : len(hrvMetrics)-1]

	avg := average(baselineSlice)
	if avg == 0 {
		return nil, nil
	}

	drop := (avg - current.Value) / avg
	if drop >= hrvDropThreshold {
		insight, err := entity.NewInsight(
			userID,
			valueobject.InsightTypeHRVDrop,
			valueobject.SeverityWarning,
			fmt.Sprintf("HRV dropped %.0f%% below 7-day baseline (current: %.0f, baseline: %.0f)", drop*100, current.Value, avg),
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		return []*entity.Insight{insight}, nil
	}

	return nil, nil
}
