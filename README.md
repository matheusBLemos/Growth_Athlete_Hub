# Growth_Athlete_Hub
Projeto Desenvolvido Para conectar aplicações de performance esportiva

## Arquitetura

- **Estado atual (as-built, pós Sprint 2):** [docs/diagrams/ARCHITECTURE_CURRENT_PT.md](docs/diagrams/ARCHITECTURE_CURRENT_PT.md) — módulos, eventos, fluxo distribuído e observabilidade, com diagramas.
- **Visão por fases (MVP → escala/IA):** [docs/diagrams/ARCHITECTURE_PLAN_PT.md](docs/diagrams/ARCHITECTURE_PLAN_PT.md).

## Observabilidade

Telemetria com OpenTelemetry (traces, métricas e logs) exportada para o Datadog
via OTLP. Desligada por padrão (disabled-safe). Veja
[docs/observability.md](docs/observability.md) para habilitar e visualizar.
