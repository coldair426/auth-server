CREATE TABLE user_project_memberships (
    user_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id UUID        NOT NULL REFERENCES oauth_clients(client_id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, client_id)
);
