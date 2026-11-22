-- name: ListComponents :many
SELECT * FROM components ORDER BY id;

-- name: InsertComponent :one
INSERT INTO components (ecosystem, name, version, purl, origin, manifest_path, kind, cpe)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id;

-- name: UpdateComponent :exec
UPDATE components
SET ecosystem = $2, name = $3, version = $4, kind = $5, cpe = $6, active = true, updated_at = now()
WHERE id = $1;

-- name: DeactivateComponents :execrows
UPDATE components
SET active = false, updated_at = now()
WHERE id = ANY(sqlc.arg(ids)::bigint[]) AND active;
