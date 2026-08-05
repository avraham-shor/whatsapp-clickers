-- name: CreateGame :one
INSERT INTO games (organizer_id, title, join_code)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListGamesByOrganizer :many
SELECT g.*, count(q.id) AS question_count
FROM games g
LEFT JOIN questions q ON q.game_id = g.id
WHERE g.organizer_id = $1
GROUP BY g.id
ORDER BY g.created_at DESC;

-- Ownership lives in the WHERE clause, always: a foreign game is
-- indistinguishable from a missing one (404, never 403).
-- name: GetGameForOrganizer :one
SELECT * FROM games
WHERE id = $1 AND organizer_id = $2;

-- name: UpdateGameScoring :one
UPDATE games
SET points_per_correct = $3,
    speed_bonus_first = $4,
    speed_bonus_second = $5,
    speed_bonus_third = $6,
    updated_at = now()
WHERE id = $1 AND organizer_id = $2
RETURNING *;

-- The AND state = 'draft' makes a concurrent double-click race-safe: only
-- one caller's UPDATE matches a row.
-- name: OpenGameLobby :one
UPDATE games
SET state = 'lobby',
    updated_at = now()
WHERE id = $1 AND organizer_id = $2 AND state = 'draft'
RETURNING *;

-- Unscoped by organizer_id on purpose: a Participant's JOIN message carries
-- no organizer context. This is architecturally distinct from the CRUD
-- endpoints' "ownership always in the WHERE clause" doctrine above — that
-- doctrine is about the organizer-facing API's 404-not-403 posture; this is
-- the participant-facing WhatsApp path, which has no ownership concept to
-- enforce.
-- name: GetGameByJoinCode :one
SELECT * FROM games WHERE join_code = $1;

-- Unscoped by organizer_id for the same reason as GetGameByJoinCode: this
-- serves RecordAnswer's post-write snapshot build (game.RecordAnswer,
-- story 3.3), another participant-facing WhatsApp path with no organizer
-- context. The caller has already validated the game/question/participant
-- via GetOpenQuestionForPlayer and RecordAnswer's own write-time guard
-- before ever reaching this read — same trust posture as PlayerRecipients'
-- unscoped ListParticipants call.
-- name: GetGameByID :one
SELECT * FROM games WHERE id = $1;

-- Starts the game: opens the question at position 1 and computes its
-- cutoff from that question's time limit. Zero rows covers both "not in
-- lobby" and "no questions" (no matching position-1 row) in one statement —
-- the engine disambiguates which one happened via a prior read, exactly
-- like OpenGameLobby disambiguates a lost race from a missing game.
-- name: StartGameFirstQuestion :one
UPDATE games g
SET state = 'question_open',
    current_question_position = 1,
    answer_cutoff_at = now() + (q.time_limit_seconds || ' seconds')::interval,
    updated_at = now()
FROM questions q
WHERE g.id = $1 AND g.organizer_id = $2 AND g.state = 'lobby'
  AND q.game_id = g.id AND q.position = 1
RETURNING g.*;

-- An early explicit close tightens the cutoff to now(); a close arriving
-- after the timer already elapsed must not push the cutoff later, hence
-- LEAST rather than a plain overwrite.
-- name: CloseCurrentQuestion :one
UPDATE games
SET state = 'question_closed',
    answer_cutoff_at = LEAST(answer_cutoff_at, now()),
    updated_at = now()
WHERE id = $1 AND organizer_id = $2 AND state = 'question_open'
RETURNING *;

-- Reveal additionally guards on every recorded answer for the current
-- question being graded (stage IS NOT NULL) — FR-16's "Reveal control
-- activates only once every received answer is graded" (epic 3.4
-- AC-3). Grading is fully synchronous through Story 3.5, so this NOT
-- EXISTS is always vacuously true today; it becomes load-bearing once
-- Story 3.6 makes AI grading genuinely async. Write-time race-safety
-- net for the engine's own CountUngradedAnswersForCurrentQuestion
-- pre-check (game/engine.go) — same "read for message accuracy,
-- write-guard for the race" discipline as every other transition here.
-- name: RevealCurrentQuestion :one
UPDATE games g
SET state = 'revealed',
    updated_at = now()
FROM questions q
WHERE g.id = $1 AND g.organizer_id = $2 AND g.state = 'question_closed'
  AND q.game_id = g.id AND q.position = g.current_question_position
  AND NOT EXISTS (SELECT 1 FROM answers a WHERE a.question_id = q.id AND a.stage IS NULL)
RETURNING g.*;

-- Same shape as StartGameFirstQuestion, guarded from 'revealed' instead of
-- 'lobby' and parameterized on the target position (current + 1) instead of
-- hardcoding 1.
-- name: OpenNextQuestion :one
UPDATE games g
SET state = 'question_open',
    current_question_position = sqlc.arg(position),
    answer_cutoff_at = now() + (q.time_limit_seconds || ' seconds')::interval,
    updated_at = now()
FROM questions q
WHERE g.id = sqlc.arg(id) AND g.organizer_id = sqlc.arg(organizer_id) AND g.state = 'revealed'
  AND q.game_id = g.id AND q.position = sqlc.arg(position)
RETURNING g.*;

-- Serves two callers: NextQuestion (when no question exists at
-- position + 1) and StopGame — one guarded write shared by both instead of
-- two near-identical queries. Resets current_question_position back to 0:
-- the migration's own invariant is that 0 means "no question open", mirroring
-- draft/lobby/finished — leaving a stale position here would make buildSnapshot
-- keep reporting a currentQuestion for a game that already ended.
-- name: FinishGame :one
UPDATE games
SET state = 'finished',
    current_question_position = 0,
    updated_at = now()
WHERE id = $1 AND organizer_id = $2 AND state IN ('question_open','question_closed','revealed')
RETURNING *;
