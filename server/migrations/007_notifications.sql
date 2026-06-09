-- Notifications history table (relational — NOT a hypertable).
-- Registra cada notificação tentada (enviada ou falha) por usuário/dispositivo,
-- para auditoria e exibição do histórico no app.
CREATE TABLE IF NOT EXISTS notifications (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    insight_id TEXT NOT NULL DEFAULT '',
    type       TEXT NOT NULL DEFAULT '',
    severity   TEXT NOT NULL DEFAULT '',
    title      TEXT NOT NULL DEFAULT '',
    body       TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL,
    error      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_created
    ON notifications(user_id, created_at);
