-- +goose Up
CREATE TABLE sources (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind        text NOT NULL CHECK (kind IN ('osv', 'kev', 'nvd', 'crtsh')),
    name        text NOT NULL,
    enabled     boolean NOT NULL DEFAULT true,
    schedule    text NOT NULL DEFAULT '',
    cursor      jsonb NOT NULL DEFAULT '{}',
    last_run_at timestamptz,
    last_ok_at  timestamptz,
    last_error  text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kind, name)
);

CREATE TABLE runs (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job         text NOT NULL,
    source_id   bigint REFERENCES sources (id) ON DELETE SET NULL,
    started_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    status      text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'succeeded', 'failed')),
    stats       jsonb NOT NULL DEFAULT '{}',
    error       text
);
CREATE INDEX runs_job_started_idx ON runs (job, started_at DESC);
CREATE INDEX runs_source_idx ON runs (source_id) WHERE source_id IS NOT NULL;

CREATE TABLE raw_items (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id    bigint NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    external_id  text NOT NULL,
    payload      jsonb NOT NULL,
    published_at timestamptz,
    fetched_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_id, external_id)
);
CREATE INDEX raw_items_published_idx ON raw_items (source_id, published_at DESC);

-- +goose Down
DROP TABLE raw_items;
DROP TABLE runs;
DROP TABLE sources;
