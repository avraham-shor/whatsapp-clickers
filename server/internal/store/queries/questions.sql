-- Every question query joins/filters through games.organizer_id so a
-- foreign question is unreachable by construction.

-- name: ListQuestionsByGame :many
SELECT q.* FROM questions q
JOIN games g ON g.id = q.game_id
WHERE q.game_id = sqlc.arg(game_id) AND g.organizer_id = sqlc.arg(organizer_id)
ORDER BY q.position, q.created_at;

-- Appends at the end of the game: position = MAX+1 (no row when the game
-- is missing or foreign, which surfaces as pgx.ErrNoRows on :one).
-- name: CreateQuestion :one
INSERT INTO questions (game_id, position, type, text, options, correct_option, accepted_answers, time_limit_seconds)
SELECT g.id,
       COALESCE((SELECT max(q.position) FROM questions q WHERE q.game_id = g.id), 0) + 1,
       sqlc.arg(type), sqlc.arg(text), sqlc.arg(options), sqlc.arg(correct_option), sqlc.arg(accepted_answers), sqlc.arg(time_limit_seconds)
FROM games g
WHERE g.id = sqlc.arg(game_id) AND g.organizer_id = sqlc.arg(organizer_id)
RETURNING *;

-- type is immutable after creation: it appears in WHERE, never in SET, so a
-- type-mismatched update matches no row.
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
RETURNING q.*;

-- name: DeleteQuestion :execrows
DELETE FROM questions q
USING games g
WHERE q.id = sqlc.arg(id)
  AND q.game_id = sqlc.arg(game_id)
  AND g.id = q.game_id
  AND g.organizer_id = sqlc.arg(organizer_id);

-- name: UpdateQuestionPosition :execrows
UPDATE questions q
SET position = sqlc.arg(position), updated_at = now()
FROM games g
WHERE q.id = sqlc.arg(id)
  AND q.game_id = sqlc.arg(game_id)
  AND g.id = q.game_id
  AND g.organizer_id = sqlc.arg(organizer_id);
