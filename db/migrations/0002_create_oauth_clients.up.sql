CREATE TABLE oauth_clients (
    client_id             UUID        PRIMARY KEY,
    name                  TEXT        NOT NULL,
    logo_url              TEXT,
    favicon_url           TEXT,
    gradient_from         TEXT        NOT NULL,
    gradient_to           TEXT        NOT NULL,
    text_dark             BOOLEAN,
    allowed_redirect_uris TEXT[]      NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
