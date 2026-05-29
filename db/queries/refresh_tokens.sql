-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (id, user_id, client_id, token_hash, expires_at, user_agent)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = now()
WHERE token_hash = $1;
