package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// ErrReorderMismatch is returned when a reorder request is not an exact
// permutation of the game's current question IDs.
var ErrReorderMismatch = errors.New("store: reorder ids do not match the game's questions")

// CreateQuestionParams carries a new question across the store boundary.
// OrganizerID scopes the insert: a missing or foreign game inserts nothing.
type CreateQuestionParams struct {
	GameID           string
	OrganizerID      string
	Type             string
	Text             string
	Options          []string
	CorrectOption    int32
	AcceptedAnswers  []string
	TimeLimitSeconds int32
}

// UpdateQuestionParams carries a question edit. Type never changes a row —
// it participates in the WHERE clause so a type-mismatched update is
// ErrNotFound, keeping the type immutable after creation.
type UpdateQuestionParams struct {
	ID               string
	GameID           string
	OrganizerID      string
	Type             string
	Text             string
	Options          []string
	CorrectOption    int32
	AcceptedAnswers  []string
	TimeLimitSeconds int32
}

// ListQuestionsByGame returns a game's questions in play order for the
// owning organizer.
func (s *Store) ListQuestionsByGame(ctx context.Context, gameID, organizerID string) ([]gen.Question, error) {
	return s.q.ListQuestionsByGame(ctx, gen.ListQuestionsByGameParams{GameID: gameID, OrganizerID: organizerID})
}

// CreateQuestion appends a question at the end of the game (position MAX+1).
func (s *Store) CreateQuestion(ctx context.Context, arg CreateQuestionParams) (gen.Question, error) {
	question, err := s.q.CreateQuestion(ctx, gen.CreateQuestionParams{
		GameID:           arg.GameID,
		OrganizerID:      arg.OrganizerID,
		Type:             arg.Type,
		Text:             arg.Text,
		Options:          arg.Options,
		CorrectOption:    arg.CorrectOption,
		AcceptedAnswers:  arg.AcceptedAnswers,
		TimeLimitSeconds: arg.TimeLimitSeconds,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Question{}, ErrNotFound
	}
	return question, err
}

// UpdateQuestion edits a question's content in place (type immutable).
func (s *Store) UpdateQuestion(ctx context.Context, arg UpdateQuestionParams) (gen.Question, error) {
	question, err := s.q.UpdateQuestion(ctx, gen.UpdateQuestionParams{
		ID:               arg.ID,
		GameID:           arg.GameID,
		OrganizerID:      arg.OrganizerID,
		Type:             arg.Type,
		Text:             arg.Text,
		Options:          arg.Options,
		CorrectOption:    arg.CorrectOption,
		AcceptedAnswers:  arg.AcceptedAnswers,
		TimeLimitSeconds: arg.TimeLimitSeconds,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Question{}, ErrNotFound
	}
	return question, err
}

// DeleteQuestion removes one question; deleting a missing or foreign
// question is ErrNotFound.
func (s *Store) DeleteQuestion(ctx context.Context, questionID, gameID, organizerID string) error {
	rows, err := s.q.DeleteQuestion(ctx, gen.DeleteQuestionParams{ID: questionID, GameID: gameID, OrganizerID: organizerID})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderQuestions rewrites positions 1..N to match orderedIDs, atomically.
// The permutation check runs inside the same transaction as the writes, so
// a concurrent insert/delete cannot slip a stale order through.
func (s *Store) ReorderQuestions(ctx context.Context, gameID, organizerID string, orderedIDs []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reorder: %w", err)
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	if _, err := q.GetGameForOrganizer(ctx, gen.GetGameForOrganizerParams{ID: gameID, OrganizerID: organizerID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	current, err := q.ListQuestionsByGame(ctx, gen.ListQuestionsByGameParams{GameID: gameID, OrganizerID: organizerID})
	if err != nil {
		return err
	}
	if len(current) != len(orderedIDs) {
		return ErrReorderMismatch
	}
	existing := make(map[string]bool, len(current))
	for _, question := range current {
		existing[question.ID] = true
	}
	for _, id := range orderedIDs {
		if !existing[id] {
			return ErrReorderMismatch
		}
		// Uniqueness within orderedIDs: consume each match once so a
		// duplicated ID cannot balance out an omitted one.
		existing[id] = false
	}

	for i, id := range orderedIDs {
		rows, err := q.UpdateQuestionPosition(ctx, gen.UpdateQuestionPositionParams{
			ID:          id,
			GameID:      gameID,
			OrganizerID: organizerID,
			Position:    int32(i + 1),
		})
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrReorderMismatch
		}
	}
	return tx.Commit(ctx)
}
