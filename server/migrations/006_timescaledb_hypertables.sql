-- 006: Converte as tabelas de telemetria em hypertables do TimescaleDB,
-- particionadas pela coluna de tempo `date`.
--
-- Requisito do TimescaleDB: toda PK / índice UNIQUE precisa incluir a coluna de
-- particionamento. Por isso recriamos aqui a PK (id -> id, date) e a unicidade
-- de external_id ((external_id) -> (external_id, date)) ANTES de criar a
-- hypertable. As migrations históricas (001/004) são mantidas intactas; numa
-- base nova as constraints originais são criadas por elas e ajustadas aqui.
--
-- IMPORTANTE p/ os repositórios: a PK passa a ser composta (id, date), mas as
-- queries por `id` sozinho (FindByID) e por `external_id` sozinho
-- (FindByExternalID) continuam funcionando — apenas deixam de usar o índice
-- mais seletivo. Nenhuma mudança de semântica nas queries é necessária.

CREATE EXTENSION IF NOT EXISTS timescaledb;

-- activities: PK e unicidade de external_id devem incluir a coluna de tempo.
ALTER TABLE activities DROP CONSTRAINT activities_pkey;
ALTER TABLE activities ADD PRIMARY KEY (id, date);
DROP INDEX IF EXISTS idx_activities_external_id;
CREATE UNIQUE INDEX idx_activities_external_id ON activities(external_id, date)
    WHERE external_id IS NOT NULL AND external_id != '';
SELECT create_hypertable('activities', 'date', if_not_exists => TRUE, migrate_data => TRUE);

-- metrics
ALTER TABLE metrics DROP CONSTRAINT metrics_pkey;
ALTER TABLE metrics ADD PRIMARY KEY (id, date);
SELECT create_hypertable('metrics', 'date', if_not_exists => TRUE, migrate_data => TRUE);

-- daily_metric_aggregates: `date` já faz parte da PK (user_id, date, metric_type).
SELECT create_hypertable('daily_metric_aggregates', 'date', if_not_exists => TRUE, migrate_data => TRUE);

-- Nota: `users`, `provider_tokens` e `device_tokens` seguem como tabelas
-- relacionais comuns (dados de negócio, baixo volume). `insights` é um
-- candidato natural a hypertable (séries por usuário/tempo), mas fica adiado
-- por simplicidade neste momento — pode ser convertido numa migration futura.
