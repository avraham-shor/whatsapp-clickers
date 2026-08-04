-- name: ListParticipants :many
SELECT * FROM participants WHERE game_id = $1 ORDER BY joined_at ASC, id ASC;

-- ON CONFLICT ... DO NOTHING with :one surfaces a conflict as
-- pgx.ErrNoRows (0 rows returned) — the same error shape every other store
-- method already checks for, no new pgconn error-code handling needed.
-- The FROM games clause re-reads game state at INSERT time rather than
-- trusting the engine's earlier read (closes the check-then-write race
-- between Join's GetGameByJoinCode and this INSERT — deferred from 2.4/2.5,
-- closed by story 3.1's code review): a state transition landing in that
-- window now makes the insert match zero source rows, which the store
-- wrapper folds into the same "no row" path as a genuine conflict.
-- allowed_states lets one query serve both callers — joinLobby passes
-- {'lobby'}, joinSpectator passes the live states.
-- name: CreateParticipant :one
INSERT INTO participants (game_id, phone, display_name, role)
SELECT g.id, sqlc.arg(phone), sqlc.arg(display_name), sqlc.arg(role)
FROM games g
WHERE g.id = sqlc.arg(game_id) AND g.state = ANY(sqlc.arg(allowed_states)::text[])
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
