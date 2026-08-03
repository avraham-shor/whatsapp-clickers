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
