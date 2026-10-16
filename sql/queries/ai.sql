-- name: GetArticleForEnrichment :one
SELECT a.id, a.title, a.content_text AS body FROM articles a WHERE a.id = @article_id;

-- name: SaveEnrichment :exec
UPDATE article_ai SET
    tldr = @tldr, key_points = @key_points::jsonb, topics = @topics::text[],
    entities = @entities::jsonb, sentiment = sqlc.narg('sentiment'),
    reading_minutes = @reading_minutes, model = @model,
    status = 'done', updated_at = now()
WHERE article_id = @article_id;

-- name: MarkEnrichmentFailed :exec
UPDATE article_ai SET status = 'failed', updated_at = now() WHERE article_id = $1;

-- name: ListNeedingEmbedding :many
-- Oldest first; skips articles that keep failing.
SELECT a.id, a.title, left(a.content_text, 4000)::text AS content_text
FROM article_ai ai JOIN articles a ON a.id = ai.article_id
WHERE ai.embedding IS NULL AND ai.embed_attempts < 3
ORDER BY ai.article_id
LIMIT $1;

-- name: SaveEmbedding :exec
UPDATE article_ai SET embedding = CAST(CAST(@embedding AS text) AS vector), updated_at = now()
WHERE article_id = @article_id;

-- name: BumpEmbedAttempts :exec
UPDATE article_ai SET embed_attempts = embed_attempts + 1 WHERE article_id = ANY(@ids::bigint[]);

-- name: ListTopics :many
SELECT t.topic::text AS topic, count(*)::bigint AS article_count
FROM articles a
JOIN subscriptions s ON s.feed_id = a.feed_id AND s.user_id = $1
JOIN article_ai ai ON ai.article_id = a.id
CROSS JOIN LATERAL unnest(ai.topics) AS t(topic)
GROUP BY t.topic
ORDER BY count(*) DESC, t.topic
LIMIT 30;
