-- +goose Up
ALTER TABLE games
    ADD COLUMN current_question_position INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN answer_cutoff_at timestamptz NOT NULL DEFAULT '1970-01-01T00:00:00Z';

-- +goose Down
ALTER TABLE games
    DROP COLUMN answer_cutoff_at,
    DROP COLUMN current_question_position;
