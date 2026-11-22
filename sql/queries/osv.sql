-- name: ListOSVComponents :many
SELECT id, purl FROM components
WHERE active AND kind = 'package' AND version <> '' AND ecosystem IN ('npm', 'PyPI', 'Go')
ORDER BY id;

-- name: CountActiveComponents :one
SELECT count(*) FROM components WHERE active;

-- name: ListRawItemsByIDs :many
SELECT external_id, payload FROM raw_items
WHERE source_id = $1 AND external_id = ANY(sqlc.arg(ids)::text[]);

-- UpsertOSVAdvisory writes the OSV view of an advisory. OSV owns severity, cvss
-- and the affected/refs elements tagged with its own OSV id. It never touches the
-- exploited fields and fills summary only when empty. Several OSV records can map
-- to one canonical id, so only elements of the same osvId are replaced.
-- name: UpsertOSVAdvisory :one
INSERT INTO advisories (id, aliases, summary, severity, cvss, affected, refs, modified_at)
VALUES (sqlc.arg(id), sqlc.arg(aliases)::text[], sqlc.arg(summary), sqlc.arg(severity),
        ROUND(sqlc.narg(cvss)::float8::numeric, 1), sqlc.arg(affected)::jsonb, sqlc.arg(refs)::jsonb, sqlc.narg(modified_at))
ON CONFLICT (id) DO UPDATE SET
    aliases     = (SELECT COALESCE(array_agg(DISTINCT a ORDER BY a), '{}') FROM unnest(advisories.aliases || EXCLUDED.aliases) a),
    summary     = CASE WHEN advisories.summary = '' THEN EXCLUDED.summary ELSE advisories.summary END,
    severity    = CASE WHEN EXCLUDED.severity = 'unknown' THEN advisories.severity ELSE EXCLUDED.severity END,
    cvss        = COALESCE(EXCLUDED.cvss, advisories.cvss),
    affected    = (SELECT COALESCE(jsonb_agg(e), '[]'::jsonb) FROM jsonb_array_elements(advisories.affected) e
                   WHERE NOT (e->>'source' = 'osv' AND e->>'osvId' = sqlc.arg(osv_id)::text)) || EXCLUDED.affected,
    refs        = (SELECT COALESCE(jsonb_agg(e), '[]'::jsonb) FROM jsonb_array_elements(advisories.refs) e
                   WHERE NOT (e->>'source' = 'osv' AND e->>'osvId' = sqlc.arg(osv_id)::text)) || EXCLUDED.refs,
    modified_at = GREATEST(advisories.modified_at, EXCLUDED.modified_at)
RETURNING (xmax = 0)::boolean AS inserted;

-- name: UpsertComponentAdvisories :execrows
INSERT INTO component_advisories (component_id, advisory_id, source)
SELECT t.c, t.a, sqlc.arg(source)::text
FROM (SELECT unnest(sqlc.arg(component_ids)::bigint[]) AS c, unnest(sqlc.arg(advisory_ids)::text[]) AS a) AS t
ON CONFLICT (component_id, advisory_id, source) DO UPDATE SET last_seen = now();
