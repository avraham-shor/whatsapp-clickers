package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

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

// RecordAnswerParams carries an already-validated, already-graded answer
// ready to persist. IsCorrect/Stage are plain bool/string (never a pointer
// or pgtype wrapper): this story's callers are fully synchronous and
// always have a definite verdict before calling — RecordAnswer converts
// them to the nullable pgtype columns the schema uses (nullable in the DB
// for Story 3.6's future async AI stage, not for this story's callers).
type RecordAnswerParams struct {
	GameID        string
	QuestionID    string
	ParticipantID string
	Response      string
	ReceivedAt    time.Time
	IsCorrect     bool
	Stage         string
}

// RecordAnswer persists one answer, already graded. A write-time guard miss
// (the question is no longer the game's current open one, the game left
// question_open, or the cutoff passed) is ErrNotFound; a genuine duplicate
// is ErrAlreadyAnswered — pgconn error-code knowledge stays inside store,
// same posture as CreateGame's join-code collision handling.
func (s *Store) RecordAnswer(ctx context.Context, arg RecordAnswerParams) (gen.Answer, error) {
	a, err := s.q.RecordAnswer(ctx, gen.RecordAnswerParams{
		QuestionID:    arg.QuestionID,
		ParticipantID: arg.ParticipantID,
		Response:      arg.Response,
		ReceivedAt:    arg.ReceivedAt,
		IsCorrect:     pgtype.Bool{Bool: arg.IsCorrect, Valid: true},
		Stage:         pgtype.Text{String: arg.Stage, Valid: true},
		GameID:        arg.GameID,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return gen.Answer{}, ErrAlreadyAnswered
	}
	// 23514 check_violation: the only reachable cause on this INSERT is a
	// stage value outside answers.stage's CHECK. grading.Stage is a plain
	// string alias, so a new stage constant compiles fine and only fails
	// here, at write time, on every single answer. Unmapped it would fall
	// through raw to game.RecordAnswer's default arm and degrade to the
	// generic Help reply with the answer silently dropped — diagnosable
	// only from a WARN line. Name the actual value instead. Stories
	// 3.5/3.6 each widen the CHECK as they add a stage; this is what tells
	// them they forgot. Code review finding, story 3.4.
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		return gen.Answer{}, fmt.Errorf("store: answer rejected by constraint %s (stage=%q): %w", pgErr.ConstraintName, arg.Stage, err)
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

// CountUngradedAnswersForCurrentQuestion returns how many recorded
// answers for gameID's current question have not yet been graded
// (stage IS NULL) — the Reveal-gate pre-check (FR-16 epic AC-3).
func (s *Store) CountUngradedAnswersForCurrentQuestion(ctx context.Context, gameID string) (int64, error) {
	return s.q.CountUngradedAnswersForCurrentQuestion(ctx, gameID)
}
