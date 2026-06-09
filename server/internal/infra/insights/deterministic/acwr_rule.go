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
	acuteWindowDays   = 7
	chronicWindowDays = 28
	acwrWarning       = 1.5
	acwrCritical      = 2.0
	acwrUnderload     = 0.8
)

type ACWRRule struct{}

func NewACWRRule() *ACWRRule {
	return &ACWRRule{}
}

func (r *ACWRRule) Evaluate(_ context.Context, userID string, metrics []*entity.Metric) ([]*entity.Insight, error) {
	loadMetrics := filterByType(metrics, valueobject.MetricTypeTrainingLoad)
	if len(loadMetrics) < chronicWindowDays {
		return nil, nil
	}

	sort.Slice(loadMetrics, func(i, j int) bool {
		return loadMetrics[i].Date.Before(loadMetrics[j].Date)
	})

	recent := loadMetrics[len(loadMetrics)-chronicWindowDays:]
	acute := recent[chronicWindowDays-acuteWindowDays:]
	chronic := recent

	acuteAvg := average(acute)
	chronicAvg := average(chronic)
	if chronicAvg == 0 {
		return nil, nil
	}

	ratio := acuteAvg / chronicAvg

	if ratio >= acwrCritical {
		insight, err := entity.NewInsight(
			userID,
			valueobject.InsightTypeOvertraining,
			valueobject.SeverityCritical,
			fmt.Sprintf("Training load dangerously high — ACWR %.2f (threshold: %.1f)", ratio, acwrCritical),
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		return []*entity.Insight{insight}, nil
	}

	if ratio >= acwrWarning {
		insight, err := entity.NewInsight(
			userID,
			valueobject.InsightTypeOvertraining,
			valueobject.SeverityWarning,
			fmt.Sprintf("Training load elevated — ACWR %.2f (threshold: %.1f)", ratio, acwrWarning),
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		return []*entity.Insight{insight}, nil
	}

	if ratio < acwrUnderload {
		insight, err := entity.NewInsight(
			userID,
			valueobject.InsightTypeUndertraining,
			valueobject.SeverityInfo,
			fmt.Sprintf("Training load below optimal — ACWR %.2f (threshold: %.1f)", ratio, acwrUnderload),
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		return []*entity.Insight{insight}, nil
	}

	return nil, nil
}
