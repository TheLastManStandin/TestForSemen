-- +goose Up
-- Optimistic concurrency for the web panel: every update increments version.
ALTER TABLE expenses ADD COLUMN version BIGINT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE expenses DROP COLUMN version;
