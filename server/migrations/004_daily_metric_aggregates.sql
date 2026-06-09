-- Daily metric aggregates (resumos diários por tipo de métrica/carga, gravados
-- pelo módulo de Processamento). A PK composta torna o reprocessamento de um
-- dia idempotente via UPSERT (ON CONFLICT).
CREATE TABLE IF NOT EXISTS daily_metric_aggregates (
    user_id     TEXT NOT NULL REFERENCES users(id),
    date        TIMESTAMPTZ NOT NULL,
    metric_type TEXT NOT NULL,
    count       INT NOT NULL DEFAULT 0,
    sum         DOUBLE PRECISION NOT NULL DEFAULT 0,
    avg         DOUBLE PRECISION NOT NULL DEFAULT 0,
    min         DOUBLE PRECISION NOT NULL DEFAULT 0,
    max         DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, date, metric_type)
);

CREATE INDEX IF NOT EXISTS idx_daily_metric_aggregates_user_date
    ON daily_metric_aggregates(user_id, date);
