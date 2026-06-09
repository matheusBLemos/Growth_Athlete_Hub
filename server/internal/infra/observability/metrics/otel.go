// Package metrics implementa port.Metrics sobre o OpenTelemetry, traduzindo as
// chamadas de negócio em instrumentos (counters/histograms) do meter global.
// Com a telemetria desabilitada, o meter global é no-op e o overhead é mínimo.
package metrics

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

const meterName = "github.com/Growth-Athlete-Hub/gah-server"

// Metrics adapta o meter do OpenTelemetry à porta port.Metrics. Os instrumentos
// são criados sob demanda e cacheados por nome (thread-safe).
type Metrics struct {
	meter      metric.Meter
	mu         sync.Mutex
	counters   map[string]metric.Int64Counter
	histograms map[string]metric.Float64Histogram
}

var _ port.Metrics = (*Metrics)(nil)

// New cria um Metrics ligado ao meter global do OpenTelemetry.
func New() *Metrics {
	return &Metrics{
		meter:      otel.Meter(meterName),
		counters:   make(map[string]metric.Int64Counter),
		histograms: make(map[string]metric.Float64Histogram),
	}
}

func (m *Metrics) IncCounter(ctx context.Context, name string, value int64, attrs ...string) {
	c := m.counter(name)
	if c == nil {
		return
	}
	c.Add(ctx, value, metric.WithAttributes(toKeyValues(attrs)...))
}

func (m *Metrics) RecordDuration(ctx context.Context, name string, d time.Duration, attrs ...string) {
	h := m.histogram(name)
	if h == nil {
		return
	}
	h.Record(ctx, float64(d.Milliseconds()), metric.WithAttributes(toKeyValues(attrs)...))
}

func (m *Metrics) counter(name string) metric.Int64Counter {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.counters[name]; ok {
		return c
	}
	c, err := m.meter.Int64Counter(name)
	if err != nil {
		return nil
	}
	m.counters[name] = c
	return c
}

func (m *Metrics) histogram(name string) metric.Float64Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()
	if h, ok := m.histograms[name]; ok {
		return h
	}
	h, err := m.meter.Float64Histogram(name, metric.WithUnit("ms"))
	if err != nil {
		return nil
	}
	m.histograms[name] = h
	return h
}

// toKeyValues converte pares chave/valor planos em attribute.KeyValue. Um par
// incompleto no final é descartado.
func toKeyValues(attrs []string) []attribute.KeyValue {
	kvs := make([]attribute.KeyValue, 0, len(attrs)/2)
	for i := 0; i+1 < len(attrs); i += 2 {
		kvs = append(kvs, attribute.String(attrs[i], attrs[i+1]))
	}
	return kvs
}
