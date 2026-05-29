-- name: GetOAuthClientByID :one
SELECT * FROM oauth_clients
WHERE client_id = $1;

-- name: GetAllowedRedirectURIs :one
SELECT allowed_redirect_uris FROM oauth_clients
WHERE client_id = $1;
