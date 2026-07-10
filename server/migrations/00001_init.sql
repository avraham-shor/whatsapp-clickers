-- +goose Up
-- No-op migration: proves the goose pipeline end-to-end.
-- Story 1.2 adds the first real schema (organizers, sessions).
SELECT 1;

-- +goose Down
SELECT 1;
