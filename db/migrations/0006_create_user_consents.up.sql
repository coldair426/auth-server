CREATE TABLE user_consents (
    id           UUID        PRIMARY KEY,
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    policy_type  TEXT        NOT NULL,
    version      TEXT        NOT NULL,
    service_id   TEXT,
    consented_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_consents_user_id ON user_consents (user_id);
