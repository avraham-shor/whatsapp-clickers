-- name: ListParticipants :many
SELECT * FROM participants WHERE game_id = $1 ORDER BY joined_at ASC, id ASC;

-- ON CONFLICT ... DO NOTHING with :one surfaces a conflict as
-- pgx.ErrNoRows (0 rows returned) — the same error shape every other store
-- method already checks for, no new pgconn error-code handling needed.
-- name: CreateParticipant :one
INSERT INTO participants (game_id, phone, display_name, role)
VALUES ($1, $2, $3, $4)
ON CONFLICT (game_id, phone) DO NOTHING
RETURNING *;

-- name: GetParticipantByPhone :one
SELECT * FROM participants WHERE game_id = $1 AND phone = $2;

-- Scoped to the phone's single most-recently-joined row across ALL games
-- (not just one game) — see story 2.4 Dev Notes' name-command scoping
-- decision. Tiebreaks on id (same as ListParticipants) since two rows can
-- share a joined_at timestamp at insert-time resolution.
-- name: UpdateParticipantNameByPhone :one
UPDATE participants
SET display_name = $2
WHERE id = (
    SELECT id FROM participants AS latest WHERE latest.phone = $1 ORDER BY latest.joined_at DESC, latest.id DESC LIMIT 1
)
RETURNING *;
