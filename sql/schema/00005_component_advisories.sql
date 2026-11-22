-- +goose Up
-- Which advisories a source reported for which component. Rows are never
-- deleted when a source stops reporting a pair; last_seen just goes stale.
CREATE TABLE component_advisories (
    component_id bigint NOT NULL REFERENCES components (id) ON DELETE CASCADE,
    advisory_id  text   NOT NULL REFERENCES advisories (id) ON DELETE CASCADE,
    source       text   NOT NULL CHECK (source IN ('osv', 'nvd')),
    first_seen   timestamptz NOT NULL DEFAULT now(),
    last_seen    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (component_id, advisory_id, source)
);
CREATE INDEX component_advisories_advisory_idx ON component_advisories (advisory_id);

-- +goose Down
DROP TABLE component_advisories;
