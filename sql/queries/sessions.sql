-- name: GetSessionUser :one
SELECT
    u.id,
    u.name,
    u.email,
    u."emailVerified",
    u.image,
    u."createdAt",
    s."expiresAt" AS session_expires_at
FROM "session" s
JOIN "user" u ON u.id = s."userId"
WHERE s.token = $1
  AND s."expiresAt" > now();
