-- name: CreateSource :one
INSERT INTO sources (kind, name, enabled, schedule)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSource :one
SELECT * FROM sources WHERE id = $1;

-- name: ListSources :many
SELECT * FROM sources ORDER BY kind, name;
