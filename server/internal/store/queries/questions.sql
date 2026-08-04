-- Every question query joins/filters through games.organizer_id so a
-- foreign question is unreachable by construction.

-- name: ListQuestionsByGame :many
SELECT q.* FROM questions q
JOIN games g ON g.id = q.game_id
WHERE q.game_id = sqlc.arg(game_id) AND g.organizer_id = sqlc.arg(organizer_id)
ORDER BY q.position, q.created_at;

-- Appends at the end of the game: position = MAX+1 (no row when the game
-- is missing, foreign, or non-draft, which surfaces as pgx.ErrNoRows on
-- :one). The draft guard closes the check-then-write race between
-- httpapi's requireDraftGame read and this INSERT (deferred from 1.4/1.5,
-- closed by story 3.1's code review — now reachable since 3.1 lets a game
-- leave draft).
-- name: CreateQuestion :one
INSERT INTO questions (game_id, position, type, text, options, correct_option, accepted_answers, time_limit_seconds)
SELECT g.id,
       COALESCE((SELECT max(q.position) FROM questions q WHERE q.game_id = g.id), 0) + 1,
       sqlc.arg(type), sqlc.arg(text), sqlc.arg(options), sqlc.arg(correct_option), sqlc.arg(accepted_answers), sqlc.arg(time_limit_seconds)
FROM games g
WHERE g.id = sqlc.arg(game_id) AND g.organizer_id = sqlc.arg(organizer_id) AND g.state = 'draft'
RETURNING *;

-- type is immutable after creation: it appears in WHERE, never in SET, so a
-- type-mismatched update matches no row. The draft guard closes the same
-- check-then-write race as CreateQuestion.
-- name: UpdateQuestion :one
UPDATE questions q
SET text = sqlc.arg(text),
    options = sqlc.arg(options),
    correct_option = sqlc.arg(correct_option),
    accepted_answers = sqlc.arg(accepted_answers),
    time_limit_seconds = sqlc.arg(time_limit_seconds),
    updated_at = now()
FROM games g
WHERE q.id = sqlc.arg(id)
  AND q.game_id = sqlc.arg(game_id)
  AND q.type = sqlc.arg(type)
  AND g.id = q.game_id
  AND g.organizer_id = sqlc.arg(organizer_id)
  AND g.state = 'draft'
RETURNING q.*;

-- Deletes one question and returns its position so the caller
-- (store/questions.go, same transaction) can close the gap it leaves —
-- StartGame/NextQuestion (story 3.1) match position by exact equality, and
-- a gap left the game permanently unstartable or silently short-circuited
-- past the gap. The draft guard closes the same check-then-write race as
-- CreateQuestion.
-- name: DeleteQuestion :one
DELETE FROM questions q
USING games g
WHERE q.id = sqlc.arg(id)
  AND q.game_id = sqlc.arg(game_id)
  AND g.id = q.game_id
  AND g.organizer_id = sqlc.arg(organizer_id)
  AND g.state = 'draft'
RETURNING q.position;

-- Closes the gap DeleteQuestion just left, run in the same transaction
-- right after it (see DeleteQuestion's comment) — every question after the
-- deleted position shifts down by one, restoring the dense 1..N invariant.
-- name: CloseQuestionPositionGap :exec
UPDATE questions
SET position = position - 1, updated_at = now()
WHERE game_id = sqlc.arg(game_id) AND position > sqlc.arg(position);

-- The draft guard closes the same check-then-write race as CreateQuestion;
-- ReorderQuestions' own transaction (store/questions.go) also re-checks the
-- game's state up front for the common case, but a state change landing
-- mid-loop is still caught here since each row's guard is evaluated fresh.
-- name: UpdateQuestionPosition :execrows
UPDATE questions q
SET position = sqlc.arg(position), updated_at = now()
FROM games g
WHERE q.id = sqlc.arg(id)
  AND q.game_id = sqlc.arg(game_id)
  AND g.id = q.game_id
  AND g.organizer_id = sqlc.arg(organizer_id)
  AND g.state = 'draft';
