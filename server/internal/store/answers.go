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

// RecordAnswerParams carries an already-validated answer ready to persist.
// IsCorrect/Stage are *bool/*string: nil means "pending AI grading" (Story
// 3.6) — a free_text answer that missed both Exact and Fuzzy is persisted
// with both nil, and game.Engine's async AI stage grades it afterward via
// UpdateAnswerGrade. The answers_grading_shape CHECK requires the two to be
// NULL/non-NULL as a pair; a caller passing one nil and the other non-nil
// is a caller bug the DB CHECK rejects, not something this layer guards.
type RecordAnswerParams struct {
	GameID        string
	QuestionID    string
	ParticipantID string
	Response      string
	ReceivedAt    time.Time
	IsCorrect     *bool
	Stage         *string
}

// RecordAnswer persists one answer, already graded. A write-time guard miss
// (the question is no longer the game's current open one, the game left
// question_open, or the cutoff passed) is ErrNotFound; a genuine duplicate
// is ErrAlreadyAnswered — pgconn error-code knowledge stays inside store,
// same posture as CreateGame's join-code collision handling.
func (s *Store) RecordAnswer(ctx context.Context, arg RecordAnswerParams) (gen.Answer, error) {
	isCorrect := pgtype.Bool{Valid: arg.IsCorrect != nil}
	if arg.IsCorrect != nil {
		isCorrect.Bool = *arg.IsCorrect
	}
	stage := pgtype.Text{Valid: arg.Stage != nil}
	if arg.Stage != nil {
		stage.String = *arg.Stage
	}
	a, err := s.q.RecordAnswer(ctx, gen.RecordAnswerParams{
		QuestionID:    arg.QuestionID,
		ParticipantID: arg.ParticipantID,
		Response:      arg.Response,
		ReceivedAt:    arg.ReceivedAt,
		IsCorrect:     isCorrect,
		Stage:         stage,
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
		return gen.Answer{}, fmt.Errorf("store: answer rejected by constraint %s (stage=%q): %w", pgErr.ConstraintName, stage.String, err)
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

// UpdateAnswerGrade persists an async grading verdict for answerID once the
// AI Semantic stage resolves (Story 3.6) — the counterpart to RecordAnswer
// leaving is_correct/stage nil for a pending row.
//
// The underlying UPDATE is guarded on stage IS NULL, so a verdict for a row
// something else already graded (the orphan sweep, or a retry that in fact
// succeeded) affects zero rows. That is a success, not an error: the
// invariant this layer promises is "the row is graded", not "this call is
// what graded it" — see queries/answers.sql for why a late verdict must not
// overwrite a final grade.
func (s *Store) UpdateAnswerGrade(ctx context.Context, answerID string, isCorrect bool, stage string) error {
	return s.q.UpdateAnswerGrade(ctx, gen.UpdateAnswerGradeParams{
		ID:        answerID,
		IsCorrect: pgtype.Bool{Bool: isCorrect, Valid: true},
		Stage:     pgtype.Text{String: stage, Valid: true},
	})
}

// FailCloseOrphanedAnswers fail-closes every answer row still ungraded
// (stage IS NULL) that was received before olderThan — a row whose AI
// grading goroutine died with its process, or whose verdict could not be
// persisted. Returns the number of rows recovered.
//
// olderThan is the caller's promise that nothing legitimately in flight can
// be that old; passing a cutoff of "now" would fail-close answers another
// live instance is still grading. See queries/answers.sql. Called at boot
// and then periodically for the process's lifetime — boot alone cannot see
// rows an outgoing instance abandons after this one started.
func (s *Store) FailCloseOrphanedAnswers(ctx context.Context, olderThan time.Time) (int64, error) {
	return s.q.FailCloseOrphanedAnswers(ctx, olderThan)
}

// AnswerForScoring is one graded answer's scoring inputs, read by
// game.Engine.Reveal before it computes points via game.AwardPoints.
type AnswerForScoring struct {
	AnswerID   string
	IsCorrect  bool
	ReceivedAt time.Time
	Seq        int64
}

// AnswerPointsParams carries one answer's already-computed point award
// (game.AwardPoints, pure, no I/O) across the store boundary — same
// "already computed, store just persists" posture as RecordAnswerParams'
// pre-graded IsCorrect/Stage.
type AnswerPointsParams struct {
	AnswerID string
	Points   int32
}

// ParticipantScore is one participant's cumulative score — the input to
// game.RankLeaderboard.
type ParticipantScore struct {
	ParticipantID string
	DisplayName   string
	Score         int32
}

// ListAnswersForScoring returns the answers to the question at
// (gameID, position) in no particular order — game.AwardPoints does its
// own sort by ReceivedAt/Seq. The question is resolved by position, not
// by id, so the caller supplies the position it intends to score
// (game.Engine.Reveal passes g.CurrentQuestionPosition).
//
// IsCorrect is read via .Bool with no .Valid check: the rows are graded
// by precondition, not by this query. Reveal's
// CountUngradedAnswersForCurrentQuestion gate has already returned zero,
// and answers_grading_shape (00011) pairs is_correct with stage, so a
// zero stage IS NULL count means every row here has a real verdict.
func (s *Store) ListAnswersForScoring(ctx context.Context, gameID string, position int32) ([]AnswerForScoring, error) {
	rows, err := s.q.ListAnswersForScoring(ctx, gen.ListAnswersForScoringParams{GameID: gameID, Position: position})
	if err != nil {
		return nil, err
	}
	out := make([]AnswerForScoring, 0, len(rows))
	for _, r := range rows {
		out = append(out, AnswerForScoring{AnswerID: r.ID, IsCorrect: r.IsCorrect.Bool, ReceivedAt: r.ReceivedAt, Seq: r.Seq.Int64})
	}
	return out, nil
}

// GetLeaderboard returns every player-role Participant's cumulative
// score for gameID, in join order (game.RankLeaderboard sorts by score
// and uses this order to break ties).
func (s *Store) GetLeaderboard(ctx context.Context, gameID string) ([]ParticipantScore, error) {
	rows, err := s.q.GetLeaderboard(ctx, gameID)
	if err != nil {
		return nil, err
	}
	out := make([]ParticipantScore, 0, len(rows))
	for _, r := range rows {
		out = append(out, ParticipantScore{ParticipantID: r.ParticipantID, DisplayName: r.DisplayName, Score: r.Score})
	}
	return out, nil
}

// AnswerResultRow is one answering Participant's phone/correctness/points
// for a just-revealed question — game.Engine.ResultsForRevealedQuestion's
// raw input (story 3.8, FR-6).
type AnswerResultRow struct {
	ParticipantID string
	Phone         string
	IsCorrect     bool
	Points        int32
}

// ListAnswerResultsForQuestion returns gameID's question-at-position
// answers, one row per answering Participant, in no particular order —
// game.Engine.ResultsForRevealedQuestion does its own leaderboard-based
// rank lookup per participant.
func (s *Store) ListAnswerResultsForQuestion(ctx context.Context, gameID string, position int32) ([]AnswerResultRow, error) {
	rows, err := s.q.ListAnswerResultsForQuestion(ctx, gen.ListAnswerResultsForQuestionParams{GameID: gameID, Position: position})
	if err != nil {
		return nil, err
	}
	out := make([]AnswerResultRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, AnswerResultRow{ParticipantID: r.ParticipantID, Phone: r.Phone, IsCorrect: r.IsCorrect.Bool, Points: r.PointsAwarded.Int32})
	}
	return out, nil
}
