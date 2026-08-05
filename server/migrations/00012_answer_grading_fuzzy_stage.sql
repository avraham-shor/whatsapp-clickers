-- +goose Up
-- Widens story 3.4's shape CHECK (answers.stage IN ('mcq','exact')) to
-- admit 'fuzzy' — this story's Hebrew fuzzy-matching stage. Story 3.6
-- (AI) widens it again to add 'ai'. is_correct/stage's shape-pairing
-- CHECK (answers_grading_shape, added in 00011) is untouched — this
-- migration only widens the allowed stage values.
ALTER TABLE answers DROP CONSTRAINT answers_stage_check;
ALTER TABLE answers ADD CONSTRAINT answers_stage_check CHECK (stage IN ('mcq', 'exact', 'fuzzy'));

-- +goose Down
-- ADD CONSTRAINT validates existing rows, and under this story's grading
-- every free-text answer records stage='fuzzy' — misses included — so from
-- the first free-text answer graded on this schema a bare re-narrowing
-- would fail with 23514 and block rollback entirely, which is exactly when
-- an operator reaches for it. Relabel first: 'exact' is the stage a
-- free-text verdict carried under story 3.4, and is_correct is left alone,
-- so the rows land in precisely the shape 3.4's code would have written.
-- A fuzzy-only MATCH is downgraded to a claim Exact would not have made,
-- which is the honest direction to lose information in — the alternative
-- is deleting a participant's answer outright. (Code review, story 3.5.)
UPDATE answers SET stage = 'exact' WHERE stage = 'fuzzy';
ALTER TABLE answers DROP CONSTRAINT answers_stage_check;
ALTER TABLE answers ADD CONSTRAINT answers_stage_check CHECK (stage IN ('mcq', 'exact'));
