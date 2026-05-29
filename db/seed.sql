-- Seed: one test oauth_client for local development / manual testing.
-- client_id: 00000000-0000-0000-0000-000000000001  (fixed UUID for easy reference)
INSERT INTO oauth_clients (
    client_id,
    name,
    logo_url,
    favicon_url,
    gradient_from,
    gradient_to,
    text_dark,
    allowed_redirect_uris
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Test App',
    NULL,
    NULL,
    '#6366f1',
    '#8b5cf6',
    false,
    ARRAY['http://localhost:3000/auth/callback']
) ON CONFLICT (client_id) DO NOTHING;
