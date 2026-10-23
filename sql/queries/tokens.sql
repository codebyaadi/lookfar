-- name: GetTokenUser :one
SELECT
    t.id AS token_id,
    t.scopes,
    u.id,
    u.name,
    u.email,
    u."emailVerified",
    u.image,
    u."createdAt"
FROM api_tokens t
JOIN "user" u ON u.id = t.user_id
WHERE t.token_hash = $1
  AND t.revoked_at IS NULL;

-- name: TouchToken :exec
UPDATE api_tokens SET last_used_at = now() WHERE id = $1;
