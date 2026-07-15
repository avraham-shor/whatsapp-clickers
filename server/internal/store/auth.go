package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// ErrNotFound is returned when a lookup matches no row. Callers outside
// store must depend on this sentinel, never on pgx internals.
var ErrNotFound = errors.New("store: not found")

// GetOrganizerByUsername returns the organizer row for a username.
func (s *Store) GetOrganizerByUsername(ctx context.Context, username string) (gen.Organizer, error) {
	org, err := s.q.GetOrganizerByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Organizer{}, ErrNotFound
	}
	return org, err
}

// UpsertOrganizer creates an organizer or replaces the password hash of an
// existing one (provisioning CLI path).
func (s *Store) UpsertOrganizer(ctx context.Context, username, passwordHash string) (gen.Organizer, error) {
	return s.q.UpsertOrganizer(ctx, gen.UpsertOrganizerParams{Username: username, PasswordHash: passwordHash})
}

// CreateSession stores a new session row keyed by the HMAC token hash.
func (s *Store) CreateSession(ctx context.Context, organizerID, tokenHash string, expiresAt time.Time) (gen.Session, error) {
	return s.q.CreateSession(ctx, gen.CreateSessionParams{
		OrganizerID: organizerID,
		TokenHash:   tokenHash,
		ExpiresAt:   expiresAt,
	})
}

// GetSessionOrganizer resolves a token hash to its organizer; expiry is
// enforced in SQL (expires_at > now()) so restarts cannot resurrect sessions.
func (s *Store) GetSessionOrganizer(ctx context.Context, tokenHash string) (gen.Organizer, error) {
	org, err := s.q.GetSessionOrganizer(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Organizer{}, ErrNotFound
	}
	return org, err
}

// DeleteSessionByTokenHash removes one session row (logout).
func (s *Store) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	return s.q.DeleteSessionByTokenHash(ctx, tokenHash)
}

// DeleteSessionsByOrganizerID removes every session an organizer holds;
// the provisioning CLI calls it so a password reset invalidates stale sessions.
func (s *Store) DeleteSessionsByOrganizerID(ctx context.Context, organizerID string) error {
	return s.q.DeleteSessionsByOrganizerID(ctx, organizerID)
}

// DeleteExpiredSessions removes stale rows; called opportunistically on login.
func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	return s.q.DeleteExpiredSessions(ctx)
}
