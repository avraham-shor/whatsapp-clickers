-- +goose Up
-- GetLeaderboard (story 3.7) LEFT JOINs answers on participant_id and runs
-- on EVERY buildSnapshot — including snapshotAfterAnswer, i.e. once per
-- inbound WhatsApp answer. The only index answers had before this one is
-- UNIQUE (question_id, participant_id) (00010), whose leading column is
-- question_id, so it cannot serve that join: every leaderboard read
-- sequentially scanned the whole table. A question-open burst multiplies
-- that by room size against a pgxpool sized max(4, numCPU) — the same
-- pool-starvation shape that forced WithMaxConcurrentAIGrades in 3.6.
-- Also the standard un-indexed-FK case (participant_id REFERENCES
-- participants(id) ON DELETE CASCADE). Code review finding, story 3.7.
--
-- Its own migration rather than an edit to 00014, where the leaderboard
-- query it serves was introduced. 00014 had already been applied to a real
-- database by the time the review found this, and goose keys applied
-- migrations by version number alone — it never re-reads a file it has
-- already run. Adding the CREATE INDEX to 00014 therefore meant every
-- database that had already applied it would silently never get the index,
-- while its Down block would fail on a DROP INDEX for something that does
-- not exist. That is not hypothetical: it is exactly what happened on the
-- dev machine, caught by re-running the E2E harness after the review's
-- patches landed.
CREATE INDEX idx_answers_participant ON answers (participant_id);

-- +goose Down
DROP INDEX idx_answers_participant;
