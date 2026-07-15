-- +goose Up
CREATE TABLE organizers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Uniqueness is case-insensitive: re-provisioning "Avraham" must update the
-- account created as "avraham", not add a second one.
CREATE UNIQUE INDEX idx_organizers_username_lower ON organizers (lower(username));

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organizer_id UUID NOT NULL REFERENCES organizers(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX idx_sessions_organizer_id ON sessions (organizer_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
DROP TABLE organizers;
