-- +goose Up
-- Widens story 3.5's shape CHECK (answers.stage IN ('mcq','exact','fuzzy'))
-- to admit 'ai' — this story's AI Semantic stage, the last stage in the
-- pipeline. answers_grading_shape (added in 00011, pairs is_correct/stage
-- nullability) is untouched by this migration — this story's new
-- "pending" shape (is_correct=NULL, stage=NULL) is already legal under
-- that CHECK; only the stage-values list widens here.
ALTER TABLE answers DROP CONSTRAINT answers_stage_check;
ALTER TABLE answers ADD CONSTRAINT answers_stage_check CHECK (stage IN ('mcq', 'exact', 'fuzzy', 'ai'));

-- +goose Down
ALTER TABLE answers DROP CONSTRAINT answers_stage_check;
ALTER TABLE answers ADD CONSTRAINT answers_stage_check CHECK (stage IN ('mcq', 'exact', 'fuzzy'));
