package deterministic

import (
	"context"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

var _ port.InsightEvaluator = (*CompositeEvaluator)(nil)

type Rule interface {
	Evaluate(ctx context.Context, userID string, metrics []*entity.Metric) ([]*entity.Insight, error)
}

type CompositeEvaluator struct {
	rules []Rule
}

func NewCompositeEvaluator(rules ...Rule) *CompositeEvaluator {
	return &CompositeEvaluator{rules: rules}
}

func (e *CompositeEvaluator) Evaluate(ctx context.Context, userID string, metrics []*entity.Metric) ([]*entity.Insight, error) {
	var all []*entity.Insight
	for _, rule := range e.rules {
		insights, err := rule.Evaluate(ctx, userID, metrics)
		if err != nil {
			return nil, err
		}
		all = append(all, insights...)
	}
	return all, nil
}
