-- +goose Up
ALTER TABLE components
    ADD COLUMN kind text NOT NULL DEFAULT 'package' CHECK (kind IN ('package', 'server', 'image', 'saas')),
    ADD COLUMN cpe  text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE components DROP COLUMN cpe, DROP COLUMN kind;
