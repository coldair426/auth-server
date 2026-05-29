-- name: InsertConsent :exec
INSERT INTO user_consents (id, user_id, policy_type, version, service_id)
VALUES ($1, $2, $3, $4, $5);

-- name: ListConsentsByUserID :many
SELECT * FROM user_consents
WHERE user_id = $1
ORDER BY consented_at ASC;
