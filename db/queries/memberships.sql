-- name: GetMembership :one
SELECT * FROM user_project_memberships
WHERE user_id = $1 AND client_id = $2;

-- name: CreateMembership :exec
INSERT INTO user_project_memberships (user_id, client_id)
VALUES ($1, $2)
ON CONFLICT (user_id, client_id) DO NOTHING;
