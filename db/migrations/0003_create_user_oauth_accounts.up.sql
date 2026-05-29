CREATE TABLE user_oauth_accounts (
    id               UUID        PRIMARY KEY,
    user_id          UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT        NOT NULL,
    provider_user_id TEXT        NOT NULL,
    email            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX idx_user_oauth_accounts_user_id ON user_oauth_accounts (user_id);
