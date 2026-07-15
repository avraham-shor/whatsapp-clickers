-- name: CreateSession :one
INSERT INTO sessions (organizer_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionOrganizer :one
SELECT organizers.* FROM sessions
JOIN organizers ON organizers.id = sessions.organizer_id
WHERE sessions.token_hash = $1 AND sessions.expires_at > now();

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions
WHERE token_hash = $1;

-- name: DeleteSessionsByOrganizerID :exec
DELETE FROM sessions
WHERE organizer_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at <= now();
