-- name: InsertArticle :exec
-- New articles get a pending article_ai row and an enrichment job in the same statement.
WITH a AS (
    INSERT INTO articles (feed_id, guid, url, title, author, published_at, summary, content_html, content_text, image_url)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    ON CONFLICT (feed_id, guid) DO NOTHING
    RETURNING id
), ai AS (
    INSERT INTO article_ai (article_id) SELECT id FROM a
)
INSERT INTO jobs (kind, payload, dedupe_key)
SELECT 'enrich_article', jsonb_build_object('article_id', id), 'enrich:' || id FROM a
ON CONFLICT (dedupe_key) DO NOTHING;

-- name: ListArticles :many
-- Cursor is (published_at, id) of the last row of the previous page.
SELECT
    a.id,
    a.feed_id,
    a.url,
    a.title,
    a.author,
    a.published_at,
    left(a.summary, 400)::text AS excerpt,
    a.image_url,
    COALESCE(s.custom_title, NULLIF(f.title, ''), f.url)::text AS feed_title,
    f.favicon AS feed_favicon,
    (ua.read_at IS NOT NULL)::boolean AS is_read,
    COALESCE(ua.saved, false)::boolean AS saved,
    COALESCE(NULLIF(ai.tldr, ''), '')::text AS tldr,
    COALESCE(ai.topics, '{}')::text[] AS topics
FROM articles a
JOIN feeds f ON f.id = a.feed_id
JOIN subscriptions s ON s.feed_id = a.feed_id AND s.user_id = @user_id
LEFT JOIN user_article ua ON ua.article_id = a.id AND ua.user_id = @user_id
LEFT JOIN article_ai ai ON ai.article_id = a.id AND ai.status = 'done'
WHERE (sqlc.narg('feed_id')::bigint IS NULL OR a.feed_id = sqlc.narg('feed_id'))
  AND (sqlc.narg('topic')::text IS NULL OR sqlc.narg('topic')::text = ANY(ai.topics))
  AND (NOT @saved_only::boolean OR ua.saved)
  AND (NOT @unread_only::boolean OR ua.read_at IS NULL)
  AND (sqlc.narg('before_published')::timestamptz IS NULL
       OR (a.published_at, a.id) < (sqlc.narg('before_published')::timestamptz, sqlc.narg('before_id')::bigint))
ORDER BY a.published_at DESC, a.id DESC
LIMIT @page_size::int;

-- name: GetArticle :one
SELECT
    a.id,
    a.feed_id,
    a.url,
    a.title,
    a.author,
    a.published_at,
    a.content_html,
    a.summary,
    a.image_url,
    COALESCE(s.custom_title, NULLIF(f.title, ''), f.url)::text AS feed_title,
    (ua.read_at IS NOT NULL)::boolean AS is_read,
    COALESCE(ua.saved, false)::boolean AS saved,
    COALESCE(ua.rating, 0)::smallint AS rating,
    COALESCE(ai.status, 'pending')::text AS ai_status,
    COALESCE(ai.tldr, '')::text AS tldr,
    COALESCE(ai.key_points, '[]'::jsonb) AS key_points,
    COALESCE(ai.topics, '{}')::text[] AS topics,
    COALESCE(ai.sentiment, '')::text AS sentiment,
    COALESCE(ai.reading_minutes, 0)::int AS reading_minutes
FROM articles a
JOIN feeds f ON f.id = a.feed_id
JOIN subscriptions s ON s.feed_id = a.feed_id AND s.user_id = @user_id
LEFT JOIN user_article ua ON ua.article_id = a.id AND ua.user_id = @user_id
LEFT JOIN article_ai ai ON ai.article_id = a.id
WHERE a.id = @id;

-- name: SetArticleState :one
-- NULL arguments leave the corresponding field unchanged.
INSERT INTO user_article (user_id, article_id, read_at, saved, rating)
SELECT @user_id, a.id,
       CASE WHEN sqlc.narg('read')::boolean IS TRUE THEN now() END,
       COALESCE(sqlc.narg('saved')::boolean, false),
       COALESCE(sqlc.narg('rating')::smallint, 0)
FROM articles a
JOIN subscriptions s ON s.feed_id = a.feed_id AND s.user_id = @user_id
WHERE a.id = @article_id
ON CONFLICT (user_id, article_id) DO UPDATE SET
    read_at = CASE sqlc.narg('read')::boolean
                  WHEN TRUE THEN COALESCE(user_article.read_at, now())
                  WHEN FALSE THEN NULL
                  ELSE user_article.read_at END,
    saved   = COALESCE(sqlc.narg('saved')::boolean, user_article.saved),
    rating  = COALESCE(sqlc.narg('rating')::smallint, user_article.rating),
    updated_at = now()
RETURNING (read_at IS NOT NULL)::boolean AS is_read, saved, rating;

-- name: MarkFeedRead :exec
INSERT INTO user_article (user_id, article_id, read_at)
SELECT @user_id, a.id, now() FROM articles a
WHERE a.feed_id = sqlc.narg('feed_id')::bigint
   OR (sqlc.narg('feed_id')::bigint IS NULL AND a.feed_id IN
        (SELECT feed_id FROM subscriptions WHERE user_id = @user_id))
ON CONFLICT (user_id, article_id) DO UPDATE SET
    read_at = COALESCE(user_article.read_at, now()), updated_at = now();

-- name: ExistingGUIDs :many
SELECT guid FROM articles WHERE feed_id = $1 AND guid = ANY(@guids::text[]);
