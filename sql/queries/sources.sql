-- name: CreateSource :one
-- The first check runs inline right after creation, so the scheduler is held
-- off for two minutes to avoid checking the same source twice at once.
INSERT INTO sources (user_id, kind, url, title, config, interval_secs, next_check_at)
VALUES (@user_id, @kind, @url, @title, @config, @interval_secs, now() + interval '2 minutes')
RETURNING *;

-- name: CountSources :one
SELECT count(*) FROM sources WHERE user_id = $1;

-- name: ListSources :many
SELECT s.*,
    (SELECT count(*) FROM snapshots sn WHERE sn.source_id = s.id)::int AS snapshot_count
FROM sources s
WHERE s.user_id = $1
ORDER BY s.created_at DESC, s.id DESC;

-- name: GetSource :one
SELECT * FROM sources WHERE id = @id AND user_id = @user_id;

-- name: GetSourceForCheck :one
SELECT * FROM sources WHERE id = $1;

-- name: UpdateSource :one
UPDATE sources SET
    title = @title,
    config = @config,
    interval_secs = @interval_secs,
    paused = @paused,
    next_check_at = CASE
        WHEN paused AND NOT @paused THEN now()
        ELSE next_check_at
    END
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: DeleteSource :execrows
DELETE FROM sources WHERE id = @id AND user_id = @user_id;

-- name: ListDueSources :many
SELECT id, next_check_at FROM sources
WHERE NOT paused AND next_check_at <= now()
ORDER BY next_check_at
LIMIT $1;

-- name: MarkSourceChecked :exec
-- Successful check: schedule the next one with up to 10% jitter.
UPDATE sources SET
    last_checked_at = now(),
    error_count = 0,
    last_error = NULL,
    title = CASE WHEN title = '' THEN @title ELSE title END,
    etag = @etag,
    last_modified = @last_modified,
    next_check_at = now() + make_interval(secs => (interval_secs * (1 + random() * 0.1))::float8)
WHERE id = @id;

-- name: MarkSourceChanged :exec
UPDATE sources SET last_changed_at = now() WHERE id = $1;

-- name: MarkSourceFailed :exec
-- Failed check: back off exponentially, capped at a day.
UPDATE sources SET
    last_checked_at = now(),
    error_count = error_count + 1,
    last_error = @last_error,
    next_check_at = now() + make_interval(
        secs => LEAST(interval_secs::bigint * (1 << LEAST(error_count + 1, 5)), 86400)::float8)
WHERE id = @id;

-- name: InsertSnapshot :one
INSERT INTO snapshots (source_id, content_hash, content, diff, added_lines, removed_lines, size_bytes)
VALUES (@source_id, @content_hash, @content, @diff, @added_lines, @removed_lines, @size_bytes)
RETURNING *;

-- name: LatestSnapshot :one
SELECT * FROM snapshots WHERE source_id = $1 ORDER BY id DESC LIMIT 1;

-- name: ListSnapshots :many
SELECT id, source_id, fetched_at, content_hash, added_lines, removed_lines, size_bytes,
    (diff IS NULL)::boolean AS baseline
FROM snapshots
WHERE source_id = @source_id
ORDER BY id DESC
LIMIT @row_limit;

-- name: GetSnapshot :one
SELECT sn.* FROM snapshots sn
JOIN sources s ON s.id = sn.source_id
WHERE sn.id = @id AND sn.source_id = @source_id AND s.user_id = @user_id;
