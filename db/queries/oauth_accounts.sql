-- name: UpsertOAuthAccount :one
INSERT INTO user_oauth_accounts (id, user_id, provider, provider_user_id, email)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (provider, provider_user_id)
DO UPDATE SET email = EXCLUDED.email
RETURNING *;

-- name: GetOAuthAccountByProvider :one
SELECT * FROM user_oauth_accounts
WHERE provider = $1 AND provider_user_id = $2;
