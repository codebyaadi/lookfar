-- name: ListInbox :many
SELECT * FROM inbox_items
WHERE user_id = @user_id
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;
