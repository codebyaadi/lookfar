-- name: CreateSource :one
INSERT INTO sources (kind, name, enabled, schedule)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSource :one
SELECT * FROM sources WHERE id = $1;

-- name: ListSources :many
SELECT * FROM sources ORDER BY kind, name;

-- name: UpsertSource :one
INSERT INTO sources (kind, name, enabled, schedule)
VALUES ($1, $2, true, $3)
ON CONFLICT (kind, name) DO UPDATE SET schedule = EXCLUDED.schedule
RETURNING *;

-- name: FinishSourceRun :exec
UPDATE sources
SET last_run_at = now(),
    last_ok_at  = CASE WHEN sqlc.arg(ok)::boolean THEN now() ELSE last_ok_at END,
    last_error  = sqlc.narg(error),
    cursor      = COALESCE(sqlc.narg(cursor)::jsonb, cursor)
WHERE id = sqlc.arg(id);
