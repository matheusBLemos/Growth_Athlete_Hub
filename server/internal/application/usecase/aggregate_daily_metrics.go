package usecase

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

// aggregatedMetricTypes são os tipos de métrica resumidos diariamente. Foco no
// training_load (carga de treino), que é o sinal central do pipeline de
// processamento; os demais entram quando há observações no dia.
var aggregatedMetricTypes = []valueobject.MetricType{
	valueobject.MetricTypeTrainingLoad,
	valueobject.MetricTypeHRV,
	valueobject.MetricTypeRestingHR,
	valueobject.MetricTypeSleepDuration,
}

type AggregateDailyMetricsInput struct {
	UserID string
	// Day é qualquer instante do dia a agregar; é normalizado para o início do
	// dia (UTC) ao computar e persistir o agregado.
	Day time.Time
}

type AggregateDailyMetricsOutput struct {
	// Count é o número de tipos de métrica que tiveram agregado escrito.
	Count int
}

// AggregateDailyMetrics computa, para um usuário e um dia, os agregados diários
// (count/sum/avg/min/max) por tipo de métrica e os faz upsert. O upsert garante
// idempotência: reprocessar o mesmo dia sobrescreve o agregado.
//
// Versão atual: agrega as métricas já gravadas (training_load, hrv, etc.). Uma
// versão mais completa adicionaria agregados derivados de atividades (ex.: carga
// somada a partir de duração×intensidade) e janelas móveis (ACWR de 7/28 dias).
type AggregateDailyMetrics struct {
	metricRepo port.MetricRepository
	aggRepo    port.AggregatedMetricRepository
}

func NewAggregateDailyMetrics(metricRepo port.MetricRepository, aggRepo port.AggregatedMetricRepository) *AggregateDailyMetrics {
	return &AggregateDailyMetrics{metricRepo: metricRepo, aggRepo: aggRepo}
}

func (uc *AggregateDailyMetrics) Execute(ctx context.Context, input AggregateDailyMetricsInput) (*AggregateDailyMetricsOutput, error) {
	if input.UserID == "" {
		return nil, entity.ErrEmptyUserID
	}

	day := startOfDayUTC(input.Day)
	dayEnd := day.Add(24*time.Hour - time.Nanosecond)

	written := 0
	for _, mt := range aggregatedMetricTypes {
		metrics, err := uc.metricRepo.FindByUserIDAndType(ctx, input.UserID, mt, day, dayEnd)
		if err != nil {
			return nil, err
		}
		if len(metrics) == 0 {
			continue
		}

		agg := &port.DailyMetricAggregate{
			UserID:     input.UserID,
			Date:       day,
			MetricType: string(mt),
		}
		for i, m := range metrics {
			agg.Count++
			agg.Sum += m.Value
			if i == 0 || m.Value < agg.Min {
				agg.Min = m.Value
			}
			if i == 0 || m.Value > agg.Max {
				agg.Max = m.Value
			}
		}
		agg.Avg = agg.Sum / float64(agg.Count)

		if err := uc.aggRepo.Upsert(ctx, agg); err != nil {
			return nil, err
		}
		written++
	}

	return &AggregateDailyMetricsOutput{Count: written}, nil
}

// startOfDayUTC normaliza um instante para 00:00:00 UTC do mesmo dia.
func startOfDayUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}
