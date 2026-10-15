-- name: UpsertFeed :one
INSERT INTO feeds (url, site_url, title, description, favicon)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (url) DO UPDATE SET
    title = CASE WHEN feeds.title = '' THEN EXCLUDED.title ELSE feeds.title END
RETURNING *;

-- name: GetFeed :one
SELECT * FROM feeds WHERE id = $1;

-- name: ClaimDueFeeds :many
-- Atomically picks feeds that are due and pushes next_fetch_at forward so
-- concurrent workers never poll the same feed.
UPDATE feeds SET next_fetch_at = now() + make_interval(secs => @lease_secs::int)
WHERE id IN (
    SELECT f.id FROM feeds f
    WHERE f.next_fetch_at <= now()
      AND EXISTS (SELECT 1 FROM subscriptions s WHERE s.feed_id = f.id)
    ORDER BY f.next_fetch_at
    LIMIT @batch::int
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkFeedFetched :exec
UPDATE feeds SET
    site_url        = COALESCE(sqlc.narg('site_url'), site_url),
    title           = CASE WHEN @title::text <> '' THEN @title::text ELSE title END,
    description     = CASE WHEN @description::text <> '' THEN @description::text ELSE description END,
    favicon         = COALESCE(sqlc.narg('favicon'), favicon),
    etag            = @etag,
    last_modified   = @last_modified,
    last_fetched_at = now(),
    next_fetch_at   = now() + make_interval(secs => @interval_secs::int),
    error_count     = 0,
    last_error      = NULL
WHERE id = @id;

-- name: MarkFeedNotModified :exec
UPDATE feeds SET
    last_fetched_at = now(),
    next_fetch_at   = now() + make_interval(secs => @interval_secs::int),
    error_count     = 0,
    last_error      = NULL
WHERE id = @id;

-- name: MarkFeedFailed :exec
UPDATE feeds SET
    error_count   = error_count + 1,
    last_error    = @last_error,
    next_fetch_at = now() + make_interval(secs => @backoff_secs::int)
WHERE id = @id;

-- name: Subscribe :exec
INSERT INTO subscriptions (user_id, feed_id, folder)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, feed_id) DO NOTHING;

-- name: Unsubscribe :execrows
DELETE FROM subscriptions WHERE user_id = $1 AND feed_id = $2;

-- name: UpdateSubscription :execrows
UPDATE subscriptions SET folder = @folder, custom_title = @custom_title
WHERE user_id = @user_id AND feed_id = @feed_id;

-- name: ListSubscriptions :many
SELECT
    f.id,
    f.url,
    f.site_url,
    COALESCE(s.custom_title, NULLIF(f.title, ''), f.url)::text AS title,
    f.favicon,
    s.folder,
    f.error_count,
    f.last_error,
    (SELECT count(*) FROM articles a
        LEFT JOIN user_article ua ON ua.article_id = a.id AND ua.user_id = s.user_id
        WHERE a.feed_id = f.id AND ua.read_at IS NULL)::bigint AS unread_count
FROM subscriptions s
JOIN feeds f ON f.id = s.feed_id
WHERE s.user_id = $1
ORDER BY lower(COALESCE(s.custom_title, NULLIF(f.title, ''), f.url));
