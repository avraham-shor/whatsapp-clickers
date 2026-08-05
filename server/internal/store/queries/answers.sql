-- Resolves the (game, question, participant) context for phone's next
-- reply — the one game where phone is a player-role Participant and the
-- question at the game's current position, REGARDLESS of the game's
-- state. Deliberately not scoped to g.state = 'question_open': a reply
-- arriving after the organizer has already closed the question (state
-- moved to question_closed/revealed, current_question_position unchanged
-- until NextQuestion) must still resolve to that question's context so
-- RecordAnswer's write-time guard below can reject it with the
-- distinguishing "closed" outcome (AC-4, EXPERIENCE.md's UJ-5 late-answer
-- example) rather than this read returning zero rows and the caller
-- degrading to the generic "no open question at all" Help reply. A
-- finished game is naturally excluded: FinishGame resets
-- current_question_position back to 0, which matches no question's
-- position. ORDER BY + LIMIT 1 is defensive belt-and-braces for a phone
-- somehow being an active player in more than one game with a current
-- question; NFR-3 assumes a single Game at a time, so this is not a real
-- multi-game feature. [ASSUMPTION]
--
-- is_open and already_answered let the caller (game.RecordAnswer) pick the
-- correct degrade reply — "closed" (AC-4) or "already answered" (AC-3) —
-- BEFORE spending effort on content-format validation (a malformed or
-- over-length reply must not get the format-hint/too-long copy just
-- because it never reached RecordAnswer's write-time guard below; review
-- finding, story 3.3). RecordAnswer's own guard remains the authority for
-- the read-then-write race window; these two columns are a best-effort
-- pre-check off the same read, not a replacement for it.
--
-- correct_option/accepted_answers carry the grading inputs (story 3.4) —
-- game.RecordAnswer needs them to compute a verdict inline, and this read
-- is already the one place that resolves the current question for phone's
-- reply; a second round trip just to re-fetch the question would be pure
-- waste.
--
-- q.created_at in the ORDER BY keeps this read's choice of question
-- identical to buildSnapshot's (questions.sql, ORDER BY q.position,
-- q.created_at) when two rows share (game_id, position). That is
-- reachable: 00003 deliberately declines UNIQUE (game_id, position) so
-- reorder can rewrite 1..N inside a transaction, and CreateQuestion's
-- max(position)+1 has no backstop against concurrent inserts. Before
-- story 3.4 a mismatch only meant a cosmetically different question row;
-- now that this SELECT also carries the grading inputs, an unordered pick
-- could grade a participant against the answer key of a question they
-- were never shown. Code review finding, story 3.4.
-- name: GetOpenQuestionForPlayer :one
SELECT g.id AS game_id, q.id AS question_id, q.type AS question_type,
       q.correct_option, q.accepted_answers, p.id AS participant_id,
       COALESCE(g.state = 'question_open' AND now() <= g.answer_cutoff_at, false)::boolean AS is_open,
       EXISTS (
           SELECT 1 FROM answers a WHERE a.question_id = q.id AND a.participant_id = p.id
       ) AS already_answered
FROM participants p
JOIN games g ON g.id = p.game_id
JOIN questions q ON q.game_id = g.id AND q.position = g.current_question_position
WHERE p.phone = sqlc.arg(phone) AND p.role = 'player'
ORDER BY g.updated_at DESC, q.created_at
LIMIT 1;

-- Persists an answer, re-validating at write time that question_id is
-- STILL the game's current open question, the game is still
-- question_open, and the cutoff has not passed — closes the
-- read-then-write race between GetOpenQuestionForPlayer above and this
-- INSERT (same "re-check the guard at write time" discipline as
-- CreateParticipant's allowed_states / CreateQuestion's state='draft').
-- Binding on the specific question_id (not re-derived via
-- current_question_position) additionally closes a narrower race: if the
-- question advances between the read and this write, current_question_position
-- would point at a DIFFERENT question, and re-deriving it here could
-- silently record the late answer against the wrong question.
--
-- Deliberately NOT `ON CONFLICT DO NOTHING`: a duplicate answer (participant
-- already has a row for this question_id) must raise the real 23505
-- unique_violation against idx_answers_question_participant, distinct from
-- a zero-source-rows guard failure. Collapsing both into one "0 rows"
-- result would make FR-7's "closed" reply and FR-8's "already answered"
-- reply indistinguishable at the store layer.
-- name: RecordAnswer :one
INSERT INTO answers (question_id, participant_id, response, received_at, is_correct, stage)
SELECT sqlc.arg(question_id), sqlc.arg(participant_id), sqlc.arg(response), sqlc.arg(received_at), sqlc.arg(is_correct), sqlc.arg(stage)
FROM games g
JOIN questions q ON q.id = sqlc.arg(question_id) AND q.game_id = g.id
WHERE g.id = sqlc.arg(game_id)
  AND g.state = 'question_open'
  AND g.current_question_position = q.position
  AND now() <= g.answer_cutoff_at
RETURNING *;

-- Live answered-count for the control panel's host-stat-pill (FR-13,
-- UX-DR11, epic AC-5). idx_answers_question_participant already leads
-- with question_id, so this is an index-only count, same reasoning as
-- the participants table's "no separate index" comment.
-- name: CountAnswersByQuestion :one
SELECT count(*) FROM answers WHERE question_id = $1;

-- Powers the engine's pre-check before attempting RevealCurrentQuestion
-- (game/engine.go Reveal) — read-check for an accurate
-- ErrGradingIncomplete vs. ErrNotQuestionClosed distinction, mirroring
-- every other transition's "read for message accuracy, write-guard for
-- the race" discipline in this codebase. RevealCurrentQuestion's own
-- NOT EXISTS guard (games.sql) is the write-time race-safety net; this
-- is the friendlier, non-transactional read.
-- name: CountUngradedAnswersForCurrentQuestion :one
SELECT count(*) FROM answers a
JOIN questions q ON q.id = a.question_id
JOIN games g ON g.id = q.game_id
WHERE g.id = sqlc.arg(game_id) AND q.position = g.current_question_position AND a.stage IS NULL;
