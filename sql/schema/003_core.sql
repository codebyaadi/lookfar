-- +goose Up
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

-- Personal API tokens for the REST API and MCP. Only the SHA-256 hash is stored.
CREATE TABLE api_tokens (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES "user" ("id") ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   BYTEA NOT NULL UNIQUE,
    scopes       TEXT[] NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX api_tokens_user_idx ON api_tokens (user_id);

-- Reports, alerts, questions and approvals. run_id gains a foreign key when
-- the runs table lands.
CREATE TABLE inbox_items (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES "user" ("id") ON DELETE CASCADE,
    run_id      BIGINT,
    origin      TEXT NOT NULL CHECK (origin IN ('agent', 'mcp', 'api')),
    kind        TEXT NOT NULL CHECK (kind IN ('report', 'alert', 'question', 'approval')),
    title       TEXT NOT NULL,
    body_md     TEXT NOT NULL DEFAULT '',
    payload     JSONB NOT NULL DEFAULT '{}',
    status      TEXT NOT NULL DEFAULT 'open'
                CHECK (status IN ('open', 'approved', 'denied', 'answered', 'dismissed', 'expired')),
    response    JSONB,
    rating      SMALLINT NOT NULL DEFAULT 0 CHECK (rating BETWEEN -1 AND 1),
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);
CREATE INDEX inbox_items_user_idx ON inbox_items (user_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE inbox_items;
DROP TABLE api_tokens;
DROP TABLE jobs;
