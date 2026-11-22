-- UpsertRawItem returns a row only when the item was inserted or its payload changed.
-- name: UpsertRawItem :one
INSERT INTO raw_items (source_id, external_id, payload, published_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (source_id, external_id) DO UPDATE
SET payload = EXCLUDED.payload, published_at = EXCLUDED.published_at, fetched_at = now()
WHERE raw_items.payload IS DISTINCT FROM EXCLUDED.payload
RETURNING (xmax = 0)::boolean AS inserted;

-- name: CountRawItems :one
SELECT count(*) FROM raw_items WHERE source_id = $1;
