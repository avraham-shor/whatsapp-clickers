-- +goose Up
CREATE TABLE games (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organizer_id UUID NOT NULL REFERENCES organizers(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    join_code TEXT NOT NULL UNIQUE,
    -- The seven canonical states are fixed project-wide (identical strings in
    -- Go/TS/DB); only 'draft' is reachable until the game engine lands (2.3/3.1).
    state TEXT NOT NULL DEFAULT 'draft' CHECK (state IN ('draft','lobby','question_open','question_closed','revealed','leaderboard','finished')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_games_organizer_id ON games (organizer_id);

CREATE TABLE questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    -- No UNIQUE (game_id, position): reorder rewrites positions 1..N inside a
    -- transaction instead of fighting a deferred constraint.
    position INTEGER NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('mcq','free_text')),
    text TEXT NOT NULL,
    -- Every column NOT NULL with a neutral default so sqlc models stay
    -- pgtype-free; the type-shape CHECK below carries the real invariants
    -- and the sentinels ('{}', 0) never reach the wire.
    options TEXT[] NOT NULL DEFAULT '{}',
    correct_option INTEGER NOT NULL DEFAULT 0,
    accepted_answers TEXT[] NOT NULL DEFAULT '{}',
    time_limit_seconds INTEGER NOT NULL DEFAULT 20 CHECK (time_limit_seconds BETWEEN 5 AND 300),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT questions_type_shape CHECK (
        (type = 'mcq' AND cardinality(options) = 4 AND correct_option BETWEEN 1 AND 4 AND cardinality(accepted_answers) = 0)
        OR
        (type = 'free_text' AND cardinality(options) = 0 AND correct_option = 0 AND cardinality(accepted_answers) >= 1)
    )
);

CREATE INDEX idx_questions_game_id_position ON questions (game_id, position);

-- +goose Down
DROP TABLE questions;
DROP TABLE games;
