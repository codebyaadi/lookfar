-- name: ClaimJob :one
-- Locks one runnable job. Jobs locked for over 10 minutes are considered
-- abandoned (worker crashed) and become claimable again.
UPDATE jobs SET locked_at = now(), attempts = attempts + 1
WHERE id = (
    SELECT j.id FROM jobs j
    WHERE j.done_at IS NULL
      AND j.run_at <= now()
      AND j.attempts < j.max_attempts
      AND j.kind = ANY(@kinds::text[])
      AND (j.locked_at IS NULL OR j.locked_at < now() - interval '10 minutes')
    ORDER BY j.run_at, j.id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: CompleteJob :exec
UPDATE jobs SET done_at = now(), locked_at = NULL, last_error = NULL WHERE id = $1;

-- name: RetryJob :exec
UPDATE jobs SET locked_at = NULL, last_error = @last_error,
    run_at = now() + make_interval(secs => @delay_secs::int)
WHERE id = @id;

-- name: DeferJob :exec
-- Rate-limited: try again later without consuming an attempt.
UPDATE jobs SET locked_at = NULL, attempts = GREATEST(attempts - 1, 0),
    run_at = now() + make_interval(secs => @delay_secs::int)
WHERE id = @id;

-- name: AbandonJob :exec
UPDATE jobs SET done_at = now(), locked_at = NULL, last_error = @last_error WHERE id = @id;
