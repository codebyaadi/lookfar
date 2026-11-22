-- UpsertKEVAdvisory writes the CISA KEV view of an advisory. KEV owns the
-- exploited fields and the array elements tagged source=kev in affected and
-- refs; it never touches severity or cvss, and only fills summary when empty.
-- name: UpsertKEVAdvisory :exec
INSERT INTO advisories (id, summary, exploited, exploited_since, affected, refs)
VALUES (sqlc.arg(id), sqlc.arg(summary), true, sqlc.arg(exploited_since), sqlc.arg(affected)::jsonb, sqlc.arg(refs)::jsonb)
ON CONFLICT (id) DO UPDATE SET
    exploited       = true,
    exploited_since = EXCLUDED.exploited_since,
    summary         = CASE WHEN advisories.summary = '' THEN EXCLUDED.summary ELSE advisories.summary END,
    affected        = (SELECT COALESCE(jsonb_agg(e), '[]'::jsonb) FROM jsonb_array_elements(advisories.affected) e
                       WHERE e->>'source' IS DISTINCT FROM 'kev') || EXCLUDED.affected,
    refs            = (SELECT COALESCE(jsonb_agg(e), '[]'::jsonb) FROM jsonb_array_elements(advisories.refs) e
                       WHERE e->>'source' IS DISTINCT FROM 'kev') || EXCLUDED.refs;

-- name: GetAdvisory :one
SELECT * FROM advisories WHERE id = $1;
