-- Device tokens table (push notification registration tokens per user/device)
CREATE TABLE IF NOT EXISTS device_tokens (
    user_id    TEXT NOT NULL REFERENCES users(id),
    token      TEXT NOT NULL,
    platform   TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (token)
);

CREATE INDEX IF NOT EXISTS idx_device_tokens_user_id
    ON device_tokens(user_id);
