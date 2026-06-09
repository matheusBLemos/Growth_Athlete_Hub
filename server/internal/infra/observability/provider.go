// Package observability faz o bootstrap do OpenTelemetry (traces + métricas) e
// expõe a infraestrutura de logs estruturados correlacionados ao trace.
//
// O código da aplicação depende apenas do OpenTelemetry — nenhuma referência à
// Datadog. A exportação é OTLP/gRPC apontando para um collector/agent local
// (o Datadog Agent, no docker-compose), mantendo o backend trocável.
//
// Princípio disabled-safe: quando Config.Enabled é false (ou não há endpoint),
// Setup instala providers no-op e retorna sem erro. A aplicação roda
// normalmente sem nenhum agent — bom para `make dev` e para os testes.
package observability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config reúne os parâmetros de telemetria. É preenchida a partir da config da
// aplicação (TOML + env). Mantida aqui para desacoplar o pacote do pacote
// config concreto.
type Config struct {
	// Enabled liga/desliga toda a telemetria. Quando false, Setup é no-op.
	Enabled bool
	// ServiceName identifica o serviço no backend (ex.: "gah-api", "gah-worker").
	ServiceName string
	// ServiceVersion é a versão/build do binário (ex.: git SHA). Opcional.
	ServiceVersion string
	// Environment é o ambiente de deploy (ex.: "dev", "staging", "prod").
	Environment string
	// OTLPEndpoint é o host:porta do collector OTLP/gRPC (ex.: "localhost:4317").
	OTLPEndpoint string
	// SampleRatio é a fração de traces amostrados (0.0–1.0). Fora do intervalo
	// cai para 1.0 (always-on), adequado a dev.
	SampleRatio float64
	// Insecure usa gRPC sem TLS (padrão para agent local). Em produção com TLS,
	// defina false.
	Insecure bool
}

// ServiceName devolve configured quando não vazio, senão fallback. Atalho para
// cada binário definir seu nome padrão (ex.: "gah-api") respeitando override por
// config/env (OTEL_SERVICE_NAME).
func ServiceName(configured, fallback string) string {
	if configured != "" {
		return configured
	}
	return fallback
}

// Setup instala os providers globais de trace e métrica do OpenTelemetry e o
// propagador de contexto (W3C TraceContext + Baggage), além das métricas de
// runtime do Go. Retorna uma função de shutdown idempotente que faz o flush dos
// exportadores — chame-a com defer no main.
//
// Quando cfg.Enabled é false ou cfg.OTLPEndpoint está vazio, instala apenas o
// propagador e devolve um shutdown no-op (sem rede, sem exportadores).
func Setup(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	// O propagador é sempre instalado: barato e necessário para que o
	// contexto de trace atravesse HTTP/AMQP mesmo com sampling agressivo.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Enabled || cfg.OTLPEndpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("observability: build resource: %w", err)
	}

	// shutdownFns acumula os flush/close de cada provider. São executados em
	// ordem reversa e os erros são agregados.
	var shutdownFns []func(context.Context) error
	shutdown = func(ctx context.Context) error {
		var errs error
		for i := len(shutdownFns) - 1; i >= 0; i-- {
			if e := shutdownFns[i](ctx); e != nil {
				errs = errors.Join(errs, e)
			}
		}
		shutdownFns = nil
		return errs
	}
	// Em caso de falha na montagem, desfaz o que já subiu.
	defer func() {
		if err != nil {
			_ = shutdown(ctx)
		}
	}()

	tp, err := newTracerProvider(ctx, cfg, res)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(tp)
	shutdownFns = append(shutdownFns, tp.Shutdown)

	mp, err := newMeterProvider(ctx, cfg, res)
	if err != nil {
		return nil, err
	}
	otel.SetMeterProvider(mp)
	shutdownFns = append(shutdownFns, mp.Shutdown)

	// Métricas de runtime do Go (GC, goroutines, heap) — baixo custo, alto valor.
	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(15 * time.Second)); err != nil {
		return nil, fmt.Errorf("observability: start runtime metrics: %w", err)
	}

	return shutdown, nil
}

// newResource descreve o serviço para o backend: nome, versão e ambiente. Esses
// atributos viram tags em traces/métricas/logs no Datadog.
func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
}

func newTracerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	exp, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("observability: otlp trace exporter: %w", err)
	}

	ratio := cfg.SampleRatio
	if ratio <= 0 || ratio > 1 {
		ratio = 1.0
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	), nil
}

func newMeterProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint)}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	exp, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("observability: otlp metric exporter: %w", err)
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp,
			sdkmetric.WithInterval(15*time.Second),
		)),
	), nil
}
