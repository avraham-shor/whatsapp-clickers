-- name: ListParticipants :many
SELECT * FROM participants WHERE game_id = $1 ORDER BY joined_at ASC, id ASC;
