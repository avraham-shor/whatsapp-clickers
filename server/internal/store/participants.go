package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// ListParticipants returns a game's participants in join order.
func (s *Store) ListParticipants(ctx context.Context, gameID string) ([]gen.Participant, error) {
	return s.q.ListParticipants(ctx, gameID)
}

// CreateParticipant inserts a participant, resolving a concurrent
// game_id+phone conflict to the existing row rather than erroring: the
// atomic INSERT ... ON CONFLICT DO NOTHING closes the check-then-write race
// window for free and reuses the exact pgx.ErrNoRows-means-"no row" idiom
// every other store method already uses. allowedStates guards the insert
// against the game having left the caller's expected phase between its
// state read and this write (deferred from 2.4/2.5, closed by story 3.1's
// code review) — a guard miss surfaces as the same zero-row path as a
// genuine conflict, so a first-time join into the "wrong" window comes back
// ErrNotFound (the engine's Join degrades that to a generic failure reply,
// same as any other unexpected error) rather than silently registering.
// created reports whether this call won the insert (false on the
// idempotent-repeat / conflict path).
func (s *Store) CreateParticipant(ctx context.Context, gameID, phone, displayName, role string, allowedStates []string) (gen.Participant, bool, error) {
	p, err := s.q.CreateParticipant(ctx, gen.CreateParticipantParams{
		GameID:        gameID,
		Phone:         phone,
		DisplayName:   displayName,
		Role:          role,
		AllowedStates: allowedStates,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, err := s.GetParticipantByPhone(ctx, gameID, phone)
		return existing, false, err
	}
	if err != nil {
		return gen.Participant{}, false, err
	}
	return p, true, nil
}

// GetParticipantByPhone returns the participant row for a phone within one
// game; a missing row is ErrNotFound.
func (s *Store) GetParticipantByPhone(ctx context.Context, gameID, phone string) (gen.Participant, error) {
	p, err := s.q.GetParticipantByPhone(ctx, gen.GetParticipantByPhoneParams{GameID: gameID, Phone: phone})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Participant{}, ErrNotFound
	}
	return p, err
}

// UpdateParticipantNameByPhone updates the phone's most-recently-joined
// participant row (across all games — see story 2.4 Dev Notes' name-command
// scoping decision). A phone with no participant row anywhere is ErrNotFound.
func (s *Store) UpdateParticipantNameByPhone(ctx context.Context, phone, displayName string) (gen.Participant, error) {
	p, err := s.q.UpdateParticipantNameByPhone(ctx, gen.UpdateParticipantNameByPhoneParams{Phone: phone, DisplayName: displayName})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Participant{}, ErrNotFound
	}
	return p, err
}
