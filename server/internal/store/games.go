package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// joinCodeCharset holds 32 unambiguous characters (no I/O/0/1 — codes are
// read aloud across a room). 32 divides 256, so sampling bytes with modulo
// stays uniform.
const joinCodeCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// joinCodeLength of 6 gives 32^6 ≈ 1.07e9 codes — collisions stay rare for
// the lifetime of the product, and the unique index catches the rest.
const joinCodeLength = 6

// createGameAttempts bounds join-code collision retries before giving up.
const createGameAttempts = 5

// NewJoinCode returns a fresh 6-character JOIN Code from the unambiguous
// charset. Pure function over crypto/rand; no database involved.
func NewJoinCode() (string, error) {
	buf := make([]byte, joinCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate join code: %w", err)
	}
	code := make([]byte, joinCodeLength)
	for i, b := range buf {
		code[i] = joinCodeCharset[int(b)%len(joinCodeCharset)]
	}
	return string(code), nil
}

// CreateGame inserts a game with a server-generated JOIN Code, retrying on
// the (unlikely) unique-violation collision. pgconn error knowledge stays
// inside store — callers never see 23505.
func (s *Store) CreateGame(ctx context.Context, organizerID, title string) (gen.Game, error) {
	for range createGameAttempts {
		code, err := NewJoinCode()
		if err != nil {
			return gen.Game{}, err
		}
		game, err := s.q.CreateGame(ctx, gen.CreateGameParams{
			OrganizerID: organizerID,
			Title:       title,
			JoinCode:    code,
		})
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			continue
		}
		if err != nil {
			return gen.Game{}, err
		}
		return game, nil
	}
	return gen.Game{}, fmt.Errorf("create game: exhausted %d join-code attempts", createGameAttempts)
}

// ListGamesByOrganizer returns the organizer's games, newest first, each
// carrying its question count.
func (s *Store) ListGamesByOrganizer(ctx context.Context, organizerID string) ([]gen.ListGamesByOrganizerRow, error) {
	return s.q.ListGamesByOrganizer(ctx, organizerID)
}

// GetGameForOrganizer returns a game only when the organizer owns it; a
// foreign game is ErrNotFound, indistinguishable from a missing one.
func (s *Store) GetGameForOrganizer(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	game, err := s.q.GetGameForOrganizer(ctx, gen.GetGameForOrganizerParams{ID: gameID, OrganizerID: organizerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}
