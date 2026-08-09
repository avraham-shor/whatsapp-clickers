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
-- q.text AS question_text (Story 3.6) carries the actual question wording
-- through to the AI Semantic stage's prompt (epic AC-1) — the only other
-- field this query was missing for that stage's inputs, since
-- correct_option/accepted_answers already made the trip in story 3.4.
-- name: GetOpenQuestionForPlayer :one
SELECT g.id AS game_id, q.id AS question_id, q.type AS question_type,
       q.text AS question_text, q.correct_option, q.accepted_answers,
       p.id AS participant_id,
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

-- Persists an async grading verdict once the AI stage resolves (Story
-- 3.6) — the first UPDATE against the answers table in this codebase
-- (00011's comment anticipated exactly this).
--
-- The stage IS NULL predicate is a write-time guard, not decoration:
-- FailCloseOrphanedAnswers below is a second writer of the same rows, so
-- "nothing else ever mutates a graded row" is false. Once a row is graded
-- — by this UPDATE, or by the sweep fail-closing it — that grade is
-- final; a verdict landing afterwards is a no-op rather than a silent
-- rewrite of a value the Organizer may already have revealed to the room
-- (and that story 3.7's scoring will read). Same "guard at write time
-- instead of trusting an earlier read" discipline as RecordAnswer's
-- INSERT. Code review finding, story 3.6.
-- name: UpdateAnswerGrade :exec
UPDATE answers SET is_correct = sqlc.arg(is_correct), stage = sqlc.arg(stage)
WHERE id = sqlc.arg(id) AND stage IS NULL;

-- Self-heals rows orphaned by a process restart/crash while their AI
-- grading goroutine was still in flight (deferred-work.md, story 3.4
-- review, "Story 3.6... two-phase NOT NULL migration, or a force-reveal
-- escape hatch" — this is the lighter of those two options). An
-- in-memory goroutine does not survive a process restart, so a crash
-- between RecordAnswer's pending INSERT and the async UPDATE leaves a
-- stage IS NULL row that nothing will ever grade again, permanently
-- blocking Reveal for that question (NFR-2 "an acknowledged answer is
-- never lost" / "degraded modes must be silent and self-healing").
--
-- received_at < older_than is what makes this safe to run while games are
-- live, and it is required, not optional. The original version was
-- unscoped and ran only at boot, on the reasoning that "a row belonging to
-- a goroutine this same process just launched cannot exist yet at boot
-- time". That holds for one process and fails for the deployment this
-- project actually uses: store/migrate.go documents that "zero-downtime
-- redeploys briefly run two instances, and both boot through this path",
-- so the booting instance's sweep would fail-close rows the *outgoing*
-- instance is still legitimately AI-grading for a currently-open question
-- — un-blocking Reveal early and announcing a wrong verdict. The caller
-- passes a cutoff comfortably beyond any legitimate in-flight grade
-- (game.OrphanAnswerAge), so only genuine orphans match.
--
-- Correspondingly the caller runs this on a ticker for the process's
-- lifetime, not just at boot: rows abandoned by an instance that dies
-- *after* the surviving instance booted are created after the only sweep a
-- boot-time-only design would ever run, and would stay pending forever.
-- Code review finding, story 3.6.
-- name: FailCloseOrphanedAnswers :one
WITH updated AS (
    UPDATE answers SET is_correct = false, stage = 'fuzzy'
    WHERE stage IS NULL AND received_at < sqlc.arg(older_than)
    RETURNING 1
)
SELECT count(*) FROM updated;

-- Answers to the Question at (game_id, position), already graded —
-- read by game.Engine.Reveal AFTER its CountUngradedAnswersForCurrentQuestion
-- pre-check confirms zero pending rows, and BEFORE the guarded
-- RevealCurrentQuestionAndAwardPoints transaction (games.sql). Safe to
-- read outside that transaction: RecordAnswer's own write-time guard
-- (state = 'question_open') means no new answer can appear for this
-- question once Reveal's caller has already observed
-- state = 'question_closed', and nothing mutates is_correct/stage
-- between the zero-pending check and this read (no other actor grades
-- an already-graded row). is_correct is read via `.Bool` without a
-- `.Valid` check downstream for the same reason — the zero-pending
-- precondition guarantees every row here is graded.
--
-- The ORDER BY q.created_at / LIMIT 1 subquery resolves (game_id,
-- position) to exactly ONE question, matching GetOpenQuestionForPlayer
-- above and buildSnapshot's ORDER BY q.position, q.created_at. Without
-- it a plain join would return the union of every question sharing that
-- position, and 00003 deliberately declines UNIQUE (game_id, position)
-- (see GetOpenQuestionForPlayer's comment: reorder rewrites 1..N inside
-- a transaction, and CreateQuestion's max(position)+1 has no backstop
-- against concurrent inserts). Unioned answers would spread the three
-- speed bonuses across two questions AND stamp points_awarded on rows
-- belonging to a question that was never revealed, breaking the
-- "non-NULL means revealed" invariant GetLeaderboard depends on. Code
-- review finding, story 3.7.
-- name: ListAnswersForScoring :many
SELECT a.id, a.is_correct, a.received_at, a.seq
FROM answers a
WHERE a.question_id = (
    SELECT q.id FROM questions q
    WHERE q.game_id = sqlc.arg(game_id) AND q.position = sqlc.arg(position)
    ORDER BY q.created_at
    LIMIT 1
);

-- Persists every answer's computed point award for one revealed question
-- in a SINGLE statement (story 3.7).
--
-- Set-based via unnest rather than one UPDATE per answer: this runs
-- inside RevealCurrentQuestionAndAwardPoints' transaction, itself inside
-- handleReveal's fixed 5s request budget. A per-row loop made the reveal
-- O(answers) sequential round-trips while holding the transaction open,
-- so at the 500-participant scale epics.md asks the design not to
-- preclude, the deadline could expire mid-loop — and because that
-- expiry is deterministic rather than transient, every retry would fail
-- identically and wedge the game at question_closed with Reveal the only
-- route out. Code review finding, story 3.7.
--
-- points_awarded IS NULL is a write-time guard, exactly mirroring
-- UpdateAnswerGrade's stage IS NULL and for the same reason: a score
-- already persisted for this answer has, by construction, already been
-- revealed to the room, so a second write is a silent rewrite of a
-- published value rather than a correction. Making the write idempotent
-- also means any future re-reveal/correction path no-ops here instead of
-- reassigning speed-bonus slots. Code review finding, story 3.7.
-- (The two single-argument unnests in a subquery SELECT list, rather
-- than the tidier two-argument `unnest(a, b) AS v(id, points)`, are
-- purely to stay inside what sqlc v1.31's catalog can analyze — it does
-- not know the multi-argument form. Postgres evaluates multiple
-- set-returning functions in a SELECT list in lockstep, and the two
-- arrays are built from the same slice, so the pairing is exact.)
-- name: UpdateAnswerPointsBatch :exec
UPDATE answers a
SET points_awarded = v.points
FROM (
    SELECT unnest(sqlc.arg(answer_ids)::uuid[]) AS id,
           unnest(sqlc.arg(points)::int[]) AS points
) AS v
WHERE a.id = v.id AND a.points_awarded IS NULL;

-- Per-answer results for gameID's just-revealed question (story 3.8) —
-- the WhatsApp personal-result dispatch's data source (FR-6). Read only
-- after Reveal's transaction has committed
-- (game.Engine.ResultsForRevealedQuestion, called from
-- httpapi/control.go's post-response goroutine, mirroring
-- dispatchQuestionOpened's placement, story 3.2): is_correct is
-- guaranteed non-NULL by Reveal's own pre-check
-- (CountUngradedAnswersForCurrentQuestion == 0 before the transaction
-- runs), and points_awarded is guaranteed non-NULL because
-- RevealCurrentQuestionAndAwardPoints (story 3.7) writes it inside the
-- same transaction that flips the game to revealed (migration 00014's
-- NULL convention) — so both are read via `.Bool`/`.Int32` without a
-- `.Valid` check downstream, same posture as ListAnswersForScoring's
-- is_correct.
--
-- The ORDER BY q.created_at / LIMIT 1 subquery resolves (game_id,
-- position) to exactly ONE question, identically to ListAnswersForScoring
-- above and for exactly the same reason: 00003 deliberately declines
-- UNIQUE (game_id, position), so a plain join on q.position returns the
-- union of every question sharing that position. Only one of those was
-- ever scored (ListAnswersForScoring resolves the same single question),
-- so the others' answer rows still carry points_awarded IS NULL — which
-- reads as 0 through `.Int32` and would send a participant a confident
-- grade for a question that was never revealed. Unlike the scoring path,
-- this one's output is an outbound WhatsApp message: unrecoverable once
-- sent. Code review finding, story 3.8 (the 3.7 review installed the
-- identical mitigation on ListAnswersForScoring; this query shipped
-- without it).
-- name: ListAnswerResultsForQuestion :many
SELECT a.participant_id, p.phone, a.is_correct, a.points_awarded
FROM answers a
JOIN participants p ON p.id = a.participant_id
WHERE a.question_id = (
    SELECT q.id FROM questions q
    WHERE q.game_id = sqlc.arg(game_id) AND q.position = sqlc.arg(position)
    ORDER BY q.created_at
    LIMIT 1
);

-- Cumulative per-participant score (FR-17/18) — sums points_awarded
-- across every revealed Question's answers; points_awarded IS NOT NULL
-- is exactly "this answer's Question has been revealed" (see migration
-- 00014). SUM(integer) is bigint in Postgres; ::int narrows back to
-- int32 to match points_per_correct's own column type. LEFT JOIN so a
-- participant with zero answers (hasn't played yet, or every answer
-- they gave is still un-revealed) still gets a 0-score row instead of
-- being absent from the leaderboard. role = 'player' excludes
-- Spectators explicitly — they never answer, but this leaderboard is
-- scoped intentionally rather than inheriting the role-blind roster gap
-- tracked elsewhere (deferred-work.md, 2.5 review). ORDER BY mirrors
-- ListParticipants' own tie-break exactly, so RankLeaderboard's stable
-- sort breaks score ties by join order.
-- name: GetLeaderboard :many
SELECT p.id AS participant_id, p.display_name, COALESCE(SUM(a.points_awarded), 0)::int AS score
FROM participants p
LEFT JOIN answers a ON a.participant_id = p.id AND a.points_awarded IS NOT NULL
WHERE p.game_id = sqlc.arg(game_id) AND p.role = 'player'
GROUP BY p.id, p.display_name
ORDER BY p.joined_at ASC, p.id ASC;

-- Per-question post-game response stats for the Organizer's results
-- summary (FR-14, story 3.10). One row per Question in the game,
-- including Questions that received no answers at all — the LEFT JOIN is
-- what makes a never-answered Question appear with 0 rather than vanish,
-- and a game stopped early leaves several of those.
--
-- Organizer-scoped through games.organizer_id, same posture as
-- ListQuestionsByGame (questions.sql): a foreign game is unreachable by
-- construction, not merely unauthorized at the handler.
--
-- correct_count deliberately uses FILTER (WHERE a.is_correct), which
-- excludes NULL: an answer still ungraded (stage IS NULL, story 3.6's
-- async AI path) counts as answered but not as correct. That is the
-- honest reading — the same row would read as a false "wrong" through a
-- bare `.Bool`, the posture deferred-work.md records against
-- ListAnswerResultsForQuestion. A finished game normally has none, since
-- Reveal is gated on zero pending grades, but a game stopped from
-- question_open can absolutely leave some.
--
-- No ORDER BY q.created_at ambiguity to resolve here, unlike
-- ListAnswersForScoring/ListAnswerResultsForQuestion: those pick ONE
-- question by position and needed a LIMIT 1 subquery to do it. This one
-- lists every question, so two rows sharing a position (00003
-- deliberately declines UNIQUE (game_id, position)) simply appear as two
-- rows, ordered identically to ListQuestionsByGame.
-- name: ListQuestionResponseStats :many
SELECT q.id AS question_id, q.position, q.type, q.text,
       count(a.id)::int AS answered_count,
       (count(a.id) FILTER (WHERE a.is_correct))::int AS correct_count
FROM questions q
JOIN games g ON g.id = q.game_id
LEFT JOIN answers a ON a.question_id = q.id
WHERE q.game_id = sqlc.arg(game_id) AND g.organizer_id = sqlc.arg(organizer_id)
GROUP BY q.id
ORDER BY q.position, q.created_at;
