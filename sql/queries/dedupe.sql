-- name: FindOverlappingAdvisories :many
SELECT * FROM advisories
WHERE id = ANY(sqlc.arg(ids)::text[]) OR aliases && sqlc.arg(ids)::text[]
ORDER BY id;

-- name: ListAdvisoriesByIDs :many
SELECT * FROM advisories WHERE id = ANY(sqlc.arg(ids)::text[]) ORDER BY id;

-- Pairs of advisory rows that share an identifier: one row's id is the other's
-- id or alias, or their aliases intersect. Joining on exploded keys keeps this
-- a hash join instead of a quadratic scan.
-- name: ListDuplicatePairs :many
WITH keys AS (
    SELECT id, id AS k FROM advisories
    UNION
    SELECT id, unnest(aliases) FROM advisories
)
SELECT DISTINCT a.id::text AS a_id, b.id::text AS b_id
FROM keys a JOIN keys b ON a.k = b.k AND a.id < b.id
ORDER BY 1, 2;

-- name: OverwriteAdvisory :exec
INSERT INTO advisories (id, aliases, summary, severity, cvss, exploited, exploited_since, affected, refs, modified_at, created_at)
VALUES (sqlc.arg(id), sqlc.arg(aliases)::text[], sqlc.arg(summary), sqlc.arg(severity),
        ROUND(sqlc.narg(cvss)::float8::numeric, 1), sqlc.arg(exploited), sqlc.narg(exploited_since),
        sqlc.arg(affected)::jsonb, sqlc.arg(refs)::jsonb, sqlc.narg(modified_at), sqlc.arg(created_at))
ON CONFLICT (id) DO UPDATE SET
    aliases = EXCLUDED.aliases, summary = EXCLUDED.summary, severity = EXCLUDED.severity, cvss = EXCLUDED.cvss,
    exploited = EXCLUDED.exploited, exploited_since = EXCLUDED.exploited_since,
    affected = EXCLUDED.affected, refs = EXCLUDED.refs, modified_at = EXCLUDED.modified_at,
    created_at = LEAST(advisories.created_at, EXCLUDED.created_at);

-- name: RepointComponentAdvisories :execrows
INSERT INTO component_advisories (component_id, advisory_id, source, first_seen, last_seen)
SELECT component_id, sqlc.arg(canonical)::text, source, min(first_seen), max(last_seen)
FROM component_advisories
WHERE advisory_id = ANY(sqlc.arg(others)::text[])
GROUP BY component_id, source
ON CONFLICT (component_id, advisory_id, source) DO UPDATE SET
    first_seen = LEAST(component_advisories.first_seen, EXCLUDED.first_seen),
    last_seen  = GREATEST(component_advisories.last_seen, EXCLUDED.last_seen);

-- name: DeleteAdvisories :execrows
DELETE FROM advisories WHERE id = ANY(sqlc.arg(ids)::text[]);
