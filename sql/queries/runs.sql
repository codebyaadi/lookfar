-- name: CreateRun :one
INSERT INTO runs (job, source_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetRun :one
SELECT * FROM runs WHERE id = $1;

-- name: ListRuns :many
SELECT * FROM runs
WHERE job = sqlc.arg(job)
ORDER BY started_at DESC, id DESC
LIMIT sqlc.arg(max_rows);

-- name: FinishRun :exec
UPDATE runs
SET finished_at = now(), status = $2, stats = $3, error = $4
WHERE id = $1;
