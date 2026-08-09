-- +goose Up
-- Room-level display setting (FR-9, story 4.1). The audience cannot set
-- prefers-reduced-motion on a projector, so the Organizer sets it for the
-- room and it rides the game snapshot. Column, not memory: a display
-- reconnect rebuilds its snapshot from this table, and losing the setting
-- on reconnect would be the one failure the auto-reconnect exists to hide.
ALTER TABLE games ADD COLUMN reduced_motion BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE games DROP COLUMN reduced_motion;
