package port

import (
	"context"
	"time"
)

// Metrics é a porta de métricas de negócio da camada de aplicação. Mantém os
// use cases desacoplados do SDK de telemetria (OpenTelemetry, etc.).
//
// attrs são pares chave/valor (ex.: "type", "running") usados como dimensões da
// métrica. Pares incompletos no final são ignorados pela implementação.
type Metrics interface {
	// IncCounter incrementa um contador monotônico identificado por name.
	IncCounter(ctx context.Context, name string, value int64, attrs ...string)
	// RecordDuration registra uma observação de duração num histograma (em ms).
	RecordDuration(ctx context.Context, name string, d time.Duration, attrs ...string)
}

// metricsCtxKey é a chave (não exportada) sob a qual o Metrics viaja no context.
type metricsCtxKey struct{}

// nopMetrics é o Metrics padrão quando nenhum foi injetado. Descarta tudo.
type nopMetrics struct{}

func (nopMetrics) IncCounter(context.Context, string, int64, ...string)             {}
func (nopMetrics) RecordDuration(context.Context, string, time.Duration, ...string) {}

// ContextWithMetrics devolve um context derivado carregando o Metrics. A borda
// da aplicação (HTTP middleware, worker) o injeta uma vez.
func ContextWithMetrics(ctx context.Context, m Metrics) context.Context {
	if m == nil {
		return ctx
	}
	return context.WithValue(ctx, metricsCtxKey{}, m)
}

// MetricsFromContext recupera o Metrics do context. Retorna um no-op quando
// nenhum foi injetado — nunca nil.
func MetricsFromContext(ctx context.Context) Metrics {
	if m, ok := ctx.Value(metricsCtxKey{}).(Metrics); ok {
		return m
	}
	return nopMetrics{}
}
