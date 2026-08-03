-- +goose Up
CREATE TABLE participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    phone TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'player' CHECK (role IN ('player','spectator')),
    joined_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (game_id, phone)
);

-- No separate idx_participants_game_id: the UNIQUE (game_id, phone)
-- constraint's index already has game_id as its leading column, which
-- serves the only query that filters on it (ListParticipants).

-- +goose Down
DROP TABLE participants;
