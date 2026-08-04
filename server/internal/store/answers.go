package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// ErrAlreadyAnswered means the participant already has a recorded answer
// for this question — RecordAnswer's duplicate path (a real 23505 unique
// violation), distinct from ErrNotFound's "write-time guard failed /
// question no longer open" path. See queries/answers.sql's RecordAnswer
// comment for why these must not collapse into one outcome.
var ErrAlreadyAnswered = errors.New("store: already answered")

// GetOpenQuestionForPlayer resolves phone's question-at-current-position
// context, regardless of the game's state (see queries/answers.sql) — a
// reply arriving after the question already closed still resolves here, so
// RecordAnswer's write-time guard can reject it with the distinguishing
// "closed" outcome. No such context anywhere is ErrNotFound.
func (s *Store) GetOpenQuestionForPlayer(ctx context.Context, phone string) (gen.GetOpenQuestionForPlayerRow, error) {
	row, err := s.q.GetOpenQuestionForPlayer(ctx, phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.GetOpenQuestionForPlayerRow{}, ErrNotFound
	}
	return row, err
}

// RecordAnswerParams carries an already-validated answer ready to persist.
type RecordAnswerParams struct {
	GameID        string
	QuestionID    string
	ParticipantID string
	Response      string
	ReceivedAt    time.Time
}

// RecordAnswer persists one answer. A write-time guard miss (the question
// is no longer the game's current open one, the game left question_open,
// or the cutoff passed) is ErrNotFound; a genuine duplicate is
// ErrAlreadyAnswered — pgconn error-code knowledge stays inside store,
// same posture as CreateGame's join-code collision handling.
func (s *Store) RecordAnswer(ctx context.Context, arg RecordAnswerParams) (gen.Answer, error) {
	a, err := s.q.RecordAnswer(ctx, gen.RecordAnswerParams{
		QuestionID:    arg.QuestionID,
		ParticipantID: arg.ParticipantID,
		Response:      arg.Response,
		ReceivedAt:    arg.ReceivedAt,
		GameID:        arg.GameID,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return gen.Answer{}, ErrAlreadyAnswered
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Answer{}, ErrNotFound
	}
	return a, err
}

// CountAnswersByQuestion returns the number of recorded answers for a
// question — the control panel's live answered-count (FR-13, epic AC-5).
func (s *Store) CountAnswersByQuestion(ctx context.Context, questionID string) (int64, error) {
	return s.q.CountAnswersByQuestion(ctx, questionID)
}
