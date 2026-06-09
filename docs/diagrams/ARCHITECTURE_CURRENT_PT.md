# Arquitetura — Estado Atual (as-built, pós Sprint 2)

Este documento descreve o que está **efetivamente implementado** hoje, após a
Sprint 2. Complementa o [ARCHITECTURE_PLAN_PT.md](ARCHITECTURE_PLAN_PT.md) — que
descreve a **visão por fases** (MVP → escala/IA) — mostrando como a Fase 1 foi
materializada, com os módulos novos (autenticação, notificações), o **worker**
de processamento assíncrono e a camada de **observabilidade**.

> Resumo das mudanças desde o plano original: foram adicionados o módulo de
> **Auth** (register/login, Argon2id, JWT), o módulo de **Notifications** (push
> FCM + histórico), o **conector Strava** completo (OAuth + webhook + sync), e
> uma camada de **observabilidade OpenTelemetry** (traces + métricas + logs)
> exportável para Datadog ou para uma stack OSS local (Grafana LGTM).

---

## 1. Visão de containers (as-built)

A aplicação é um **monolito modular** com dois binários: a **API** (HTTP, Fiber)
e o **Worker** (consumidor de eventos do RabbitMQ). Ambos compartilham os mesmos
use cases e adapters (Clean Architecture) e auto-aplicam as migrations no boot.

```mermaid
graph TB
    subgraph Clientes
        CLI["Web / Mobile"]
    end

    subgraph Externos
        STRAVA["Strava — API + Webhooks"]
        FCM["FCM HTTP v1 (push)"]
    end

    subgraph API["cmd/api — Fiber (HTTP :8080)"]
        AUTH["Auth ✨<br/>register · login · Argon2id · JWT"]
        INGEST["Ingestion<br/>POST /activities · POST /metrics"]
        QUERY["Query<br/>GET /metrics (cache-aside)"]
        INS["Insights<br/>POST /insights/generate"]
        CONN["Connectors / Strava<br/>connect · sync · callback · webhook"]
        NOTApi["Notifications ✨<br/>devices · GET /notifications"]
    end

    subgraph WK["cmd/worker — Consumers"]
        PROC["Processing<br/>pipeline de raw activities"]
        NOTw["Notifications ✨<br/>push on insight.generated"]
        SWH["Strava Webhook<br/>resolve athlete → sync"]
    end

    MQ{{"RabbitMQ<br/>exchange gah.events (topic)<br/>+ DLX / DLQ"}}

    PG[("PostgreSQL + TimescaleDB<br/>users · activities* · metrics*<br/>insights · provider_tokens<br/>daily_metric_aggregates* · device_tokens<br/>notifications<br/>asterisco = hypertable")]
    REDIS[("Redis — cache-aside")]

    CLI --> AUTH & INGEST & QUERY & INS & CONN & NOTApi
    STRAVA -->|webhook / OAuth| CONN

    AUTH --> PG
    INGEST --> PG
    INGEST --> MQ
    QUERY --> REDIS
    QUERY --> PG
    INS --> PG
    CONN --> PG
    CONN -->|raw.activity.imported| MQ
    NOTApi --> PG

    MQ -->|raw.activity.imported| PROC
    PROC --> PG
    PROC -->|insight.generated| MQ
    MQ -->|insight.generated| NOTw
    NOTw --> PG
    NOTw --> FCM
    MQ -->|strava.webhook.activity| SWH
    SWH -->|raw.activity.imported| MQ

    classDef novo fill:#e6ffe6,stroke:#33aa33;
    class AUTH,NOTApi,NOTw novo;
```

✨ = módulos/peças **adicionados na Sprint 2**.

---

## 2. Módulos implementados

| Módulo | Onde roda | Responsabilidade |
|---|---|---|
| **Auth** ✨ | API | Registro/login, hashing Argon2id (+pepper), emissão/validação de JWT, middleware de proteção de rotas. |
| **Ingestion** | API | Entrada REST de atividades e métricas; valida e publica eventos. |
| **Query** | API | Leitura de métricas com cache-aside no Redis (invalidação por versão). |
| **Insights** | API + Worker | Regras determinísticas (HRV, FC repouso, sono, ACWR, recuperação). |
| **Connectors / Strava** | API + Worker | OAuth2, refresh de token, sync de atividades, verificação e recebimento de webhook; publica `raw.activity.imported`. |
| **Processing** | Worker | Pipeline: validação → dedup (idempotência por external_id) → normalização → persistência → agregação diária → insights → publica `insight.generated`. |
| **Notifications** ✨ | Worker | Consome `insight.generated`, dispara push (FCM HTTP v1, ou LogNotifier como fallback) e grava histórico. Registro de dispositivos via API. |

---

## 3. Eventos (RabbitMQ — exchange `gah.events`)

Mensageria por topic exchange, com dead-letter exchange/queues por handler. O
corpo é o envelope `port.Event{Type, Payload}` (JSON) e a routing key é o `Type`.

| Evento (routing key) | Produtor | Consumidor | Observação |
|---|---|---|---|
| `user.registered` | Auth (API) | — | seam para futuro (boas-vindas, etc.) |
| `activity.registered` | Ingestion (API) | — | seam para futuro |
| `metric.recorded` | Ingestion (API) | — | seam para futuro |
| `raw.activity.imported` | Connectors / Strava | **Processing** (Worker) | entrada do pipeline |
| `insight.generated` | Processing (Worker) | **Notifications** (Worker) | dispara push |
| `strava.webhook.activity` | Strava webhook (API) | **Strava Webhook** (Worker) | resolve atleta → sync |

---

## 4. Fluxo distribuído (Strava → insight → push)

Exemplo ponta a ponta. O **contexto de trace (W3C `traceparent`) é propagado
pelos headers AMQP**, então API e Worker compartilham o mesmo `trace_id` — um
único trace distribuído atravessa o broker.

```mermaid
sequenceDiagram
    autonumber
    participant S as Strava
    participant API as gah-api (Connectors)
    participant MQ as RabbitMQ
    participant W as gah-worker
    participant DB as Postgres
    participant FCM as FCM

    S->>API: POST webhook (activity)
    API->>MQ: publish strava.webhook.activity  [traceparent]
    MQ->>W: consume (Strava Webhook)
    W->>S: GET atividades novas (sync)
    W->>MQ: publish raw.activity.imported  [traceparent]
    MQ->>W: consume (Processing)
    W->>DB: persiste atividade + agrega métricas
    W->>W: gera insights (regras)
    W->>MQ: publish insight.generated  [traceparent]
    MQ->>W: consume (Notifications)
    W->>DB: grava histórico de notificação
    W->>FCM: push para dispositivos do usuário
    Note over API,W: mesmo trace_id em todo o caminho (propagação AMQP)
```

---

## 5. Observabilidade (camada nova) ✨

Os dois binários são instrumentados com **OpenTelemetry** (neutro a fornecedor).
A app **só fala OTLP** — o destino (Datadog ou stack OSS) é um detalhe de infra,
trocável por configuração. Tudo é *disabled-safe*: com `OBSERVABILITY_ENABLED=false`
os providers são no-op (sem rede, sem agent).

**Instrumentação:** HTTP via `otelfiber`; Postgres via `XSAM/otelsql`; Redis via
`redisotel`; **AMQP por propagação manual** (inject/extract de `traceparent` nos
headers → trace distribuído). Métricas de runtime do Go + counters de negócio
(`gah.activities.registered`, `gah.metrics.recorded`, `gah.insights.generated`,
`gah.notifications.sent`, histograma `gah.raw_activity.process.duration`). Logs
estruturados em `slog` JSON (stdout) com `trace_id`/`span_id` (OTel) e
`dd.trace_id`/`dd.span_id` (Datadog) para correlação log↔trace.

```mermaid
graph LR
    subgraph APP["gah-api + gah-worker — OpenTelemetry SDK"]
        TR["Traces<br/>otelfiber · otelsql · redisotel · AMQP"]
        ME["Métricas<br/>runtime + gah.*"]
        LO["Logs<br/>slog JSON → stdout"]
    end

    subgraph OSS["Profile dev-observability (open source, local)"]
        COL["OTel Collector<br/>(grafana/otel-lgtm)"]
        TEMPO[("Tempo — traces")]
        PROM[("Prometheus — métricas")]
        LOKI[("Loki — logs")]
        ALLOY["Grafana Alloy<br/>(tail de container logs)"]
        GRAF["Grafana UI :3000"]
        COL --> TEMPO & PROM
        ALLOY --> LOKI
        TEMPO & PROM & LOKI --> GRAF
    end

    subgraph DD["Profile observability (SaaS)"]
        AGENT["Datadog Agent<br/>(OTLP + logs)"]
        DDOG["Datadog (APM · Logs · Metrics)"]
        AGENT --> DDOG
    end

    TR -->|OTLP/gRPC :4317| COL
    ME -->|OTLP/gRPC :4317| COL
    LO -.docker logs.-> ALLOY

    TR -.OTLP/gRPC.-> AGENT
    ME -.OTLP/gRPC.-> AGENT
    LO -.tail.-> AGENT
```

Detalhes operacionais e queries em [../observability.md](../observability.md).

---

## 6. Performance / tuning

Carga sintética revelou que, sob alta concorrência, o gargalo é a **saturação do
pool de conexões do Postgres** (não as queries). O pool passou a ser **elástico**
(`max_open_conns` 50, `max_idle_conns` 25, `conn_max_idle_time` 90s): cresce sob
carga e libera conexões ociosas. Ferramenta de carga e veredito em
`manual_tests/sprint_2/` (local; ver README da sprint).

---

## 7. Correspondência com o plano por fases

O as-built é a **Fase 1** do [plano](ARCHITECTURE_PLAN_PT.md) materializada, com
dois acréscimos que o plano não detalhava no diagrama de Fase 1: **Auth** (base
para os recursos profissionais relacionais previstos nas fases seguintes) e
**Notifications** (o "push notifications simples" citado como suficiente para o
MVP). A observabilidade é transversal e acompanha a plataforma em todas as fases.
Os pontos de evolução (Kafka, Cassandra, Realtime, IA/RAG) permanecem como
descrito no plano — trocas de adapter atrás das mesmas ports.
