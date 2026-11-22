-- +goose Up
-- id is the canonical advisory identifier (for example CVE-2026-1234 or GHSA-xxxx).
CREATE TABLE advisories (
    id              text PRIMARY KEY,
    aliases         text[] NOT NULL DEFAULT '{}',
    summary         text NOT NULL DEFAULT '',
    severity        text NOT NULL DEFAULT 'unknown' CHECK (severity IN ('unknown', 'low', 'medium', 'high', 'critical')),
    cvss            numeric(3, 1),
    exploited       boolean NOT NULL DEFAULT false,
    exploited_since timestamptz,
    affected        jsonb NOT NULL DEFAULT '[]',
    refs            jsonb NOT NULL DEFAULT '[]',
    modified_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX advisories_aliases_idx ON advisories USING GIN (aliases);
CREATE INDEX advisories_exploited_idx ON advisories (exploited_since DESC) WHERE exploited;

CREATE TABLE components (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ecosystem     text NOT NULL DEFAULT '',
    name          text NOT NULL,
    version       text NOT NULL DEFAULT '',
    purl          text NOT NULL,
    origin        text NOT NULL CHECK (origin IN ('manifest', 'manual')),
    manifest_path text NOT NULL DEFAULT '',
    active        boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (purl, origin, manifest_path)
);
CREATE INDEX components_purl_idx ON components (purl);
CREATE INDEX components_active_idx ON components (ecosystem, name) WHERE active;

CREATE TABLE domains (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    domain     text NOT NULL UNIQUE,
    kind       text NOT NULL CHECK (kind IN ('owned', 'brand_term')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE certificates (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ct_id      text NOT NULL UNIQUE,
    names      text[] NOT NULL DEFAULT '{}',
    issuer     text NOT NULL DEFAULT '',
    not_before timestamptz,
    seen_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX certificates_names_idx ON certificates USING GIN (names);
CREATE INDEX certificates_seen_idx ON certificates (seen_at DESC);

-- subject is the advisory id or certificate id; target is the component or domain id.
-- Both are text because they point at different tables depending on module.
CREATE TABLE findings (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    module        text NOT NULL CHECK (module IN ('advisory', 'lookalike')),
    subject       text NOT NULL,
    target        text NOT NULL,
    score         integer NOT NULL DEFAULT 0 CHECK (score BETWEEN 0 AND 100),
    severity      text NOT NULL DEFAULT 'medium' CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    status        text NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'relevant', 'irrelevant', 'suppressed')),
    reason        text NOT NULL DEFAULT '',
    reason_source text NOT NULL DEFAULT 'rules' CHECK (reason_source IN ('rules', 'model')),
    backfill      boolean NOT NULL DEFAULT false,
    first_seen    timestamptz NOT NULL DEFAULT now(),
    notified_at   timestamptz,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (module, subject, target)
);
CREATE INDEX findings_status_first_seen_idx ON findings (status, first_seen DESC);
CREATE INDEX findings_unnotified_idx ON findings (first_seen) WHERE notified_at IS NULL;

-- +goose Down
DROP TABLE findings;
DROP TABLE certificates;
DROP TABLE domains;
DROP TABLE components;
DROP TABLE advisories;
