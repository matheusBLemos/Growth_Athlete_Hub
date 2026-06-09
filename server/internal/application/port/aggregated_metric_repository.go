package port

import (
	"context"
	"time"
)

// DailyMetricAggregate é o agregado diário de uma métrica/carga de um usuário.
// É a forma persistida na tabela daily_metric_aggregates: chave composta por
// (UserID, Date, MetricType). Os campos numéricos resumem as observações do dia.
type DailyMetricAggregate struct {
	UserID     string
	Date       time.Time // normalizada para o início do dia (UTC)
	MetricType string    // ex.: "training_load", "activity_count"
	Count      int       // número de observações agregadas no dia
	Sum        float64   // soma dos valores
	Avg        float64   // média dos valores (Sum/Count)
	Min        float64   // menor valor observado
	Max        float64   // maior valor observado
}

// AggregatedMetricRepository persiste agregados diários. Upsert garante
// idempotência: reprocessar o mesmo dia sobrescreve o agregado em vez de
// duplicá-lo (PK em (user_id, date, metric_type)).
type AggregatedMetricRepository interface {
	Upsert(ctx context.Context, agg *DailyMetricAggregate) error
	Find(ctx context.Context, userID string, date time.Time, metricType string) (*DailyMetricAggregate, error)
}
