-- +goose Up
CREATE TABLE answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    participant_id UUID NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    -- Normalized: "1".."4" for MCQ (never the raw letter — story 3.4's
    -- grading compares this directly against questions.correct_option),
    -- trimmed free text for free_text. No CHECK tying this to the
    -- question's type — the FK alone can't express it, and the write path
    -- (game.RecordAnswer) is the only writer and already enforces it.
    response TEXT NOT NULL,
    -- No DEFAULT now(): the engine always supplies this explicitly, from
    -- the Go-process webhook-ingestion clock (wa.InboundMessage.ReceivedAt),
    -- not the DB transaction clock — FR-7's authoritative receipt time.
    -- Resolves the two-clocks item deferred from story 2.1's second-round
    -- review (see this story's Dev Notes).
    received_at timestamptz NOT NULL,
    -- Monotonic tie-break for Speed Bonus ordering (FR-17, story 3.7) when
    -- two answers share a received_at at timestamptz resolution. Not read
    -- by this story.
    seq BIGINT GENERATED ALWAYS AS IDENTITY
);

-- Named to match architecture's own Naming Patterns example verbatim
-- ("Indexes: idx_<table>_<cols> (idx_answers_question_participant)").
-- Enforces FR-8 (one recorded answer per participant per question) at the
-- DB level; a duplicate INSERT raises a real 23505 unique_violation
-- rather than silently matching zero rows — see RecordAnswer below, which
-- depends on that distinction. No separate idx_answers_question index:
-- this index already leads with question_id (participants table
-- precedent, migration 00008).
CREATE UNIQUE INDEX idx_answers_question_participant ON answers (question_id, participant_id);

-- +goose Down
DROP TABLE answers;
