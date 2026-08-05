-- +goose Up
-- Nullable now even though this story's write path (game.RecordAnswer)
-- always supplies both together, synchronously — forward-compatible
-- with Story 3.6, where an AI verdict may not be known at INSERT time
-- and a row could (in that story's design) be inserted ungraded and
-- updated once the async call resolves. The shape CHECK keeps the two
-- columns from ever drifting into a half-graded state in the meantime.
ALTER TABLE answers
    ADD COLUMN is_correct BOOLEAN,
    ADD COLUMN stage TEXT CHECK (stage IN ('mcq','exact')),
    ADD CONSTRAINT answers_grading_shape CHECK ((is_correct IS NULL) = (stage IS NULL));

-- Backfill every answer story 3.3 already recorded. Without this, each
-- pre-existing row keeps (NULL, NULL) — which the shape CHECK happily
-- accepts, so the migration succeeds — and is then counted as ungraded
-- forever by CountUngradedAnswersForCurrentQuestion and
-- RevealCurrentQuestion's NOT EXISTS guard. Since no UPDATE against
-- answers exists anywhere in the codebase (grading is INSERT-time only),
-- any game sitting on a question with pre-3.4 answers could never be
-- revealed again, and Reveal is the only route to 'revealed' and thus to
-- NextQuestion. Code review finding, story 3.4.
--
-- The two CASE arms mirror grading.GradeMCQ and grading.GradeExact
-- exactly: MCQ compares the stored response digit against correct_option,
-- Exact tests literal membership in accepted_answers. questions_type_shape
-- guarantees type is one of these two, so the ELSE arm is unreachable.
UPDATE answers a
SET stage = CASE WHEN q.type = 'mcq' THEN 'mcq' ELSE 'exact' END,
    is_correct = CASE
        WHEN q.type = 'mcq' THEN a.response = q.correct_option::text
        ELSE a.response = ANY (q.accepted_answers)
    END
FROM questions q
WHERE q.id = a.question_id AND a.stage IS NULL;

-- +goose Down
ALTER TABLE answers
    DROP CONSTRAINT answers_grading_shape,
    DROP COLUMN stage,
    DROP COLUMN is_correct;
