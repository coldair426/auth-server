-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at)
VALUES ($1, now(), now())
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;
