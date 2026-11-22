-- name: ListNVDServerComponents :many
SELECT id, name, version FROM components
WHERE active AND kind = 'server' AND version <> ''
ORDER BY id;

-- Candidate rows are pre-filtered with LIKE (underscore is a wildcard there);
-- the exact vendor:product test happens in Go.
-- name: ListRawItemsLike :many
SELECT external_id, payload FROM raw_items
WHERE source_id = $1 AND payload::text LIKE sqlc.arg(pattern)::text;

-- name: ListEnrichmentCandidates :many
SELECT a.id::text AS advisory_id,
       (CASE WHEN a.id LIKE 'CVE-%' THEN a.id
             ELSE (SELECT min(x) FROM unnest(a.aliases) x WHERE x LIKE 'CVE-%') END)::text AS cve_id
FROM advisories a
WHERE (a.severity = 'unknown' OR a.cvss IS NULL)
  AND EXISTS (SELECT 1 FROM component_advisories ca WHERE ca.advisory_id = a.id)
  AND (a.id LIKE 'CVE-%' OR EXISTS (SELECT 1 FROM unnest(a.aliases) x WHERE x LIKE 'CVE-%'))
ORDER BY a.exploited DESC, a.id;

-- UpsertNVDAdvisory writes the NVD view of an advisory. NVD only fills gaps:
-- severity when it is unknown, cvss when it is empty, summary when empty.
-- It owns the affected/refs elements tagged source=nvd and never touches the
-- exploited fields.
-- name: UpsertNVDAdvisory :one
INSERT INTO advisories (id, summary, severity, cvss, affected, refs, modified_at)
VALUES (sqlc.arg(id), sqlc.arg(summary), sqlc.arg(severity), ROUND(sqlc.narg(cvss)::float8::numeric, 1),
        sqlc.arg(affected)::jsonb, sqlc.arg(refs)::jsonb, sqlc.narg(modified_at))
ON CONFLICT (id) DO UPDATE SET
    summary     = CASE WHEN advisories.summary = '' THEN EXCLUDED.summary ELSE advisories.summary END,
    severity    = CASE WHEN advisories.severity = 'unknown' THEN EXCLUDED.severity ELSE advisories.severity END,
    cvss        = COALESCE(advisories.cvss, EXCLUDED.cvss),
    affected    = (SELECT COALESCE(jsonb_agg(e), '[]'::jsonb) FROM jsonb_array_elements(advisories.affected) e
                   WHERE e->>'source' IS DISTINCT FROM 'nvd') || EXCLUDED.affected,
    refs        = (SELECT COALESCE(jsonb_agg(e), '[]'::jsonb) FROM jsonb_array_elements(advisories.refs) e
                   WHERE e->>'source' IS DISTINCT FROM 'nvd') || EXCLUDED.refs,
    modified_at = GREATEST(advisories.modified_at, EXCLUDED.modified_at)
RETURNING (xmax = 0)::boolean AS inserted;
