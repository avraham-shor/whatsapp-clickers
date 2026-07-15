-- name: GetOrganizerByUsername :one
SELECT * FROM organizers
WHERE lower(username) = lower(sqlc.arg(username));

-- name: UpsertOrganizer :one
INSERT INTO organizers (username, password_hash)
VALUES ($1, $2)
ON CONFLICT (lower(username)) DO UPDATE SET password_hash = EXCLUDED.password_hash
RETURNING *;
