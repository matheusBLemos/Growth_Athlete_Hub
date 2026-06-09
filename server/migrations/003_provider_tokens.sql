-- Provider tokens table (OAuth credentials for external connectors, e.g. Strava)
CREATE TABLE IF NOT EXISTS provider_tokens (
    user_id       TEXT NOT NULL REFERENCES users(id),
    provider      TEXT NOT NULL,
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL DEFAULT '',
    expires_at    TIMESTAMPTZ,
    scope         TEXT NOT NULL DEFAULT '',
    athlete_id    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_provider_tokens_provider_athlete
    ON provider_tokens(provider, athlete_id);
