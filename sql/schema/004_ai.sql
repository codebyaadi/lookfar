-- +goose Up
-- Shared AI output: one row per article, regardless of how many users follow it.
CREATE TABLE article_ai (
    article_id      BIGINT PRIMARY KEY REFERENCES articles (id) ON DELETE CASCADE,
    tldr            TEXT NOT NULL DEFAULT '',
    key_points      JSONB NOT NULL DEFAULT '[]',
    topics          TEXT[] NOT NULL DEFAULT '{}',
    entities        JSONB NOT NULL DEFAULT '[]',
    sentiment       TEXT,
    reading_minutes INT NOT NULL DEFAULT 0,
    model           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'failed')),
    embedding       vector(1024),
    embed_attempts  INT NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX article_ai_topics_idx ON article_ai USING GIN (topics);
CREATE INDEX article_ai_embedding_idx ON article_ai USING hnsw (embedding vector_cosine_ops);
CREATE INDEX article_ai_needs_embedding_idx ON article_ai (article_id) WHERE embedding IS NULL;

-- Postgres-backed job queue (FOR UPDATE SKIP LOCKED).
CREATE TABLE jobs (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind         TEXT NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}',
    dedupe_key   TEXT UNIQUE,
    run_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    locked_at    TIMESTAMPTZ,
    done_at      TIMESTAMPTZ,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX jobs_runnable_idx ON jobs (run_at, id) WHERE done_at IS NULL;

-- Daily token accounting. scope is 'enrich' or 'user:<id>' (chat quotas).
CREATE TABLE ai_usage (
    day           DATE NOT NULL,
    scope         TEXT NOT NULL,
    requests      INT NOT NULL DEFAULT 0,
    input_tokens  BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (day, scope)
);

-- Backfill: enqueue every existing article (throttled by the worker's rate limiter).
INSERT INTO article_ai (article_id) SELECT id FROM articles;
INSERT INTO jobs (kind, payload, dedupe_key)
SELECT 'enrich_article', jsonb_build_object('article_id', id), 'enrich:' || id FROM articles;

-- +goose Down
DROP TABLE ai_usage;
DROP TABLE jobs;
DROP TABLE article_ai;
