-- Users table (PostgreSQL — relational/business data)
CREATE TABLE IF NOT EXISTS users (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    birth_date TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Activities table (TimescaleDB hypertable candidate)
CREATE TABLE IF NOT EXISTS activities (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id),
    type          TEXT NOT NULL,
    date          TIMESTAMPTZ NOT NULL,
    duration_ns   BIGINT NOT NULL,
    avg_heart_rate INT NOT NULL DEFAULT 0,
    external_id   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_activities_external_id
    ON activities(external_id) WHERE external_id IS NOT NULL AND external_id != '';

CREATE INDEX IF NOT EXISTS idx_activities_user_date
    ON activities(user_id, date);

-- Metrics table (TimescaleDB hypertable candidate)
CREATE TABLE IF NOT EXISTS metrics (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    type       TEXT NOT NULL,
    value      DOUBLE PRECISION NOT NULL,
    date       TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_metrics_user_type_date
    ON metrics(user_id, type, date);

-- Insights table
CREATE TABLE IF NOT EXISTS insights (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    type       TEXT NOT NULL,
    severity   TEXT NOT NULL,
    message    TEXT NOT NULL,
    date       TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_insights_user_date
    ON insights(user_id, date);
