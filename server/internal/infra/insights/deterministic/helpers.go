package deterministic

import (
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

func filterByType(metrics []*entity.Metric, metricType valueobject.MetricType) []*entity.Metric {
	var result []*entity.Metric
	for _, m := range metrics {
		if m.Type == metricType {
			result = append(result, m)
		}
	}
	return result
}

func average(metrics []*entity.Metric) float64 {
	if len(metrics) == 0 {
		return 0
	}
	var sum float64
	for _, m := range metrics {
		sum += m.Value
	}
	return sum / float64(len(metrics))
}
