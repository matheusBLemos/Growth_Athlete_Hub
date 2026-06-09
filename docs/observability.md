# Observabilidade (OpenTelemetry → Datadog)

O GAH é instrumentado com **OpenTelemetry** (neutro a fornecedor) nos três
pilares — **traces**, **métricas** e **logs**. A exportação é OTLP/gRPC para um
**Datadog Agent**; o código não depende da Datadog, só do OTel. Trocar de backend
significa trocar o Agent, não o código.

## Princípio: disabled-safe

Tudo é controlado por `OBSERVABILITY_ENABLED` (padrão `false`). Desligado, a
aplicação instala providers **no-op**: nenhuma conexão OTLP, nenhum agent
necessário. `make dev` e os testes rodam sem Datadog. Os logs estruturados em
JSON saem em stdout de qualquer forma.

## O que é coletado

| Pilar | Origem | Exemplos |
|-------|--------|----------|
| Traces | otelfiber (HTTP), XSAM/otelsql (Postgres), redisotel (Redis), propagação AMQP manual | `POST /activities` ligando Fiber → Postgres → Redis; trace **distribuído** API → RabbitMQ → worker |
| Métricas | runtime do Go, otelsql (pool), otelfiber (HTTP), counters de negócio | `gah.activities.registered`, `gah.metrics.recorded`, `gah.insights.generated`, `gah.notifications.sent`, `gah.raw_activity.process.duration` |
| Logs | `slog` JSON em stdout, coletado pelo Agent | correlacionados ao trace por `trace_id`/`span_id` (OTel) e `dd.trace_id`/`dd.span_id` (Datadog) |

## Variáveis de ambiente

| Var | Padrão | Descrição |
|-----|--------|-----------|
| `OBSERVABILITY_ENABLED` | `false` | Liga traces + métricas. |
| `OTEL_SERVICE_NAME` | `gah-api` / `gah-worker` | Nome do serviço (cada binário define o seu). |
| `OTEL_SERVICE_VERSION` | vazio | Versão/build (ex.: git SHA). |
| `DD_ENV` | `dev` | Ambiente de deploy. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | Endpoint OTLP/gRPC (host:porta, **sem** scheme). |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | gRPC sem TLS (agent local). |
| `OTEL_TRACES_SAMPLER_ARG` | `1.0` | Fração de traces amostrados (0.0–1.0). |
| `LOG_LEVEL` | `info` | Nível mínimo de log (`debug`/`info`/`warn`/`error`). |

Para o Datadog Agent: `DD_API_KEY` (obrigatória) e `DD_SITE` (padrão
`datadoghq.com`; use `datadoghq.eu`, `us5.datadoghq.com`, etc. conforme a região).

## Rodando localmente com o Agent

O `datadog-agent` está num profile `observability` do compose, então **não** sobe
por padrão:

```bash
export DD_API_KEY=<sua-api-key>
export OBSERVABILITY_ENABLED=true
# opcional: export DD_SITE=datadoghq.eu

docker compose -f deploy/docker-compose.yml --profile observability up --build
```

Gere tráfego para popular o Datadog:

```bash
make seed                                   # 2 usuários de teste
# login -> POST /api/v1/auth/login (maria.atleta@example.com / SenhaForte123)
# POST /api/v1/activities, POST /api/v1/insights/generate, ...
```

No Datadog você verá:

- **APM**: o trace `POST /activities` (Fiber → Postgres → Redis) e o trace
  distribuído da API ao worker passando pelo RabbitMQ.
- **Logs**: correlacionados ao trace (clique de log → trace e vice-versa).
- **Metrics**: runtime do Go, pool do banco e os counters `gah.*`.

### Dev com `go run` no host

A porta `4317` do agent é exposta no host, então é possível rodar a app fora do
compose exportando para o agent:

```bash
docker compose -f deploy/docker-compose.yml --profile observability up -d datadog-agent db rabbitmq redis
cd server && OBSERVABILITY_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 go run ./cmd/api
```

## Arquitetura no código

- `internal/infra/observability/provider.go` — bootstrap do OTel (`Setup`), no-op safe.
- `internal/infra/observability/logging/` — `port.Logger` sobre `slog` + injeção de trace.
- `internal/infra/observability/metrics/` — `port.Metrics` sobre o meter OTel.
- `internal/application/port/{logger,metrics}.go` — portas + helpers de context.
- Instrumentação: `internal/infra/http/` (otelfiber + middleware),
  `internal/infra/persistence/postgres/db.go` (otelsql),
  `internal/infra/cache/redis/cache.go` (redisotel),
  `internal/infra/messaging/rabbitmq/` (propagação AMQP).

`Logger` e `Metrics` são injetados no `context` na borda (HTTP middleware e
contexto base do worker) e recuperados pelos use cases via
`port.LoggerFromContext(ctx)` / `port.MetricsFromContext(ctx)` — sem acoplar a
camada de aplicação à infraestrutura.
