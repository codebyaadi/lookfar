-- +goose Up
CREATE TABLE digests (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    period_start  timestamptz NOT NULL,
    period_end    timestamptz NOT NULL,
    finding_count integer NOT NULL DEFAULT 0,
    sent_at       timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE deliveries (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    finding_id bigint REFERENCES findings (id) ON DELETE CASCADE,
    digest_id  bigint REFERENCES digests (id) ON DELETE CASCADE,
    channel    text NOT NULL CHECK (channel IN ('email', 'webhook')),
    status     text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts   integer NOT NULL DEFAULT 0,
    last_error text,
    sent_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (num_nonnulls(finding_id, digest_id) = 1)
);
CREATE INDEX deliveries_finding_idx ON deliveries (finding_id) WHERE finding_id IS NOT NULL;
CREATE INDEX deliveries_digest_idx ON deliveries (digest_id) WHERE digest_id IS NOT NULL;
CREATE INDEX deliveries_pending_idx ON deliveries (created_at) WHERE status <> 'sent';

-- Single-user installs: tokens and sessions belong to the one admin. Only hashes are stored.
CREATE TABLE api_tokens (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name         text NOT NULL,
    token_hash   bytea NOT NULL UNIQUE,
    scopes       text[] NOT NULL DEFAULT '{}',
    last_used_at timestamptz,
    revoked_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash   bytea NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
DROP TABLE api_tokens;
DROP TABLE deliveries;
DROP TABLE digests;
