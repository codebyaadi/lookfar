-- +goose Up
-- Things to watch. Every kind is normalised to lines of text so one diff
-- engine serves pages, feeds and GitHub releases.
CREATE TABLE sources (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES "user" ("id") ON DELETE CASCADE,
    kind            TEXT NOT NULL CHECK (kind IN ('rss', 'page', 'github')),
    url             TEXT NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    config          JSONB NOT NULL DEFAULT '{}',
    interval_secs   INT NOT NULL DEFAULT 3600 CHECK (interval_secs >= 300),
    next_check_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    etag            TEXT,
    last_modified   TEXT,
    last_checked_at TIMESTAMPTZ,
    last_changed_at TIMESTAMPTZ,
    error_count     INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    paused          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, kind, url)
);
CREATE INDEX sources_due_idx ON sources (next_check_at) WHERE NOT paused;
CREATE INDEX sources_user_idx ON sources (user_id, created_at DESC);

-- A new row is stored only when normalised content changed, so a source's
-- snapshots are its version history. The first row is the baseline (no diff).
CREATE TABLE snapshots (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id    BIGINT NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    content_hash TEXT NOT NULL,
    content      TEXT NOT NULL,
    diff         JSONB,
    added_lines  INT NOT NULL DEFAULT 0,
    removed_lines INT NOT NULL DEFAULT 0,
    size_bytes   INT NOT NULL
);
CREATE INDEX snapshots_source_idx ON snapshots (source_id, id DESC);

-- +goose Down
DROP TABLE snapshots;
DROP TABLE sources;
