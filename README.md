# Growth_Athlete_Hub
Projeto Desenvolvido Para conectar aplicações de performance esportiva

## Arquitetura

- **Estado atual (as-built, pós Sprint 2):** [PT](docs/diagrams/ARCHITECTURE_CURRENT_PT.md) · [EN](docs/diagrams/ARCHITECTURE_CURRENT.md) — módulos, eventos, fluxo distribuído e observabilidade, com diagramas.
- **Visão por fases (MVP → escala/IA):** [PT](docs/diagrams/ARCHITECTURE_PLAN_PT.md) · [EN](docs/diagrams/ARCHITECTURE_PLAN.md).

## Observabilidade

Telemetria com OpenTelemetry (traces, métricas e logs) exportada para o Datadog
via OTLP. Desligada por padrão (disabled-safe). Veja
[docs/observability.md](docs/observability.md) para habilitar e visualizar.
