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

// UpdateGameScoringParams carries a full scoring replacement for one game
// (FR-17 configuration half): all four values are set on every call.
type UpdateGameScoringParams struct {
	GameID           string
	OrganizerID      string
	PointsPerCorrect int32
	SpeedBonusFirst  int32
	SpeedBonusSecond int32
	SpeedBonusThird  int32
}

// UpdateGameScoring replaces the game's scoring configuration; ownership is
// in the WHERE clause, so a foreign game is ErrNotFound like a missing one.
func (s *Store) UpdateGameScoring(ctx context.Context, arg UpdateGameScoringParams) (gen.Game, error) {
	game, err := s.q.UpdateGameScoring(ctx, gen.UpdateGameScoringParams{
		ID:               arg.GameID,
		OrganizerID:      arg.OrganizerID,
		PointsPerCorrect: arg.PointsPerCorrect,
		SpeedBonusFirst:  arg.SpeedBonusFirst,
		SpeedBonusSecond: arg.SpeedBonusSecond,
		SpeedBonusThird:  arg.SpeedBonusThird,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}

// UpdateGameDisplaySettings sets the room-level display settings;
// ownership is in the WHERE clause, so a foreign game is ErrNotFound
// like a missing one.
func (s *Store) UpdateGameDisplaySettings(ctx context.Context, gameID, organizerID string, reducedMotion bool) (gen.Game, error) {
	game, err := s.q.UpdateGameDisplaySettings(ctx, gen.UpdateGameDisplaySettingsParams{ID: gameID, OrganizerID: organizerID, ReducedMotion: reducedMotion})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}

// OpenGameLobby transitions a game from draft to lobby; a foreign/missing
// game or one no longer in draft (including a concurrent racer that already
// won the transition) is ErrNotFound — the game package turns the latter
// case into its own ErrNotDraft.
func (s *Store) OpenGameLobby(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	game, err := s.q.OpenGameLobby(ctx, gen.OpenGameLobbyParams{ID: gameID, OrganizerID: organizerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}

// GetGameByJoinCode looks up a game by its JOIN Code, unscoped by organizer:
// the WhatsApp JOIN path carries no organizer context. A missing/unknown
// code is ErrNotFound.
func (s *Store) GetGameByJoinCode(ctx context.Context, joinCode string) (gen.Game, error) {
	game, err := s.q.GetGameByJoinCode(ctx, joinCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}

// GetGameByID looks up a game by its ID, unscoped by organizer — same
// posture as GetGameByJoinCode, for RecordAnswer's post-write snapshot
// build (another participant-facing WhatsApp path). A missing game is
// ErrNotFound.
func (s *Store) GetGameByID(ctx context.Context, gameID string) (gen.Game, error) {
	game, err := s.q.GetGameByID(ctx, gameID)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}

// StartGameFirstQuestion transitions a game from lobby to question_open on
// its first question; a foreign/missing game, one not in lobby, or one with
// no questions (including a concurrent racer that already won the
// transition) is ErrNotFound — the game package disambiguates which.
func (s *Store) StartGameFirstQuestion(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	game, err := s.q.StartGameFirstQuestion(ctx, gen.StartGameFirstQuestionParams{ID: gameID, OrganizerID: organizerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}

// CloseCurrentQuestion transitions a game from question_open to
// question_closed; a foreign/missing game or one not question_open
// (including a lost race) is ErrNotFound.
func (s *Store) CloseCurrentQuestion(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	game, err := s.q.CloseCurrentQuestion(ctx, gen.CloseCurrentQuestionParams{ID: gameID, OrganizerID: organizerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}

// RevealCurrentQuestionAndAwardPoints transitions gameID from
// question_closed to revealed AND persists each answer's points_awarded
// for the question just revealed, atomically (FR-17, story 3.7) — the
// state transition and the scoring write happen together or not at all,
// so points_awarded can never be non-NULL for a question the state
// machine says isn't revealed, or vice versa. points is already
// computed by game.AwardPoints (pure, no I/O) — this method only
// persists what it's given, same posture as RecordAnswer's pre-graded
// IsCorrect/Stage. A foreign/missing game, or one not question_closed
// (including a lost race), is ErrNotFound.
//
// The scoring write is ONE set-based statement (UpdateAnswerPointsBatch,
// queries/answers.sql) rather than a loop, so the transaction costs two
// round-trips regardless of participant count — see that query's comment
// for why an O(answers) loop inside handleReveal's 5s budget was a
// deterministic wedge rather than a mere slowdown.
func (s *Store) RevealCurrentQuestionAndAwardPoints(ctx context.Context, gameID, organizerID string, points []AnswerPointsParams) (gen.Game, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return gen.Game{}, fmt.Errorf("begin reveal: %w", err)
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	g, err := q.RevealCurrentQuestion(ctx, gen.RevealCurrentQuestionParams{ID: gameID, OrganizerID: organizerID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return gen.Game{}, ErrNotFound
		}
		return gen.Game{}, err
	}
	if len(points) > 0 {
		ids := make([]string, len(points))
		awards := make([]int32, len(points))
		for i, p := range points {
			ids[i] = p.AnswerID
			awards[i] = p.Points
		}
		if err := q.UpdateAnswerPointsBatch(ctx, gen.UpdateAnswerPointsBatchParams{
			AnswerIds: ids,
			Points:    awards,
		}); err != nil {
			return gen.Game{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return gen.Game{}, err
	}
	return g, nil
}

// ShowLeaderboard transitions a game from revealed to leaderboard (FR-13,
// FR-18, story 4.5); a foreign/missing game or one not revealed (including a
// lost race) is ErrNotFound. current_question_position deliberately does not
// move — the Leaderboard is a pause on the question just revealed.
func (s *Store) ShowLeaderboard(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	game, err := s.q.ShowLeaderboard(ctx, gen.ShowLeaderboardParams{ID: gameID, OrganizerID: organizerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}

// OpenNextQuestion transitions a game from revealed or leaderboard to
// question_open on the question at position; a foreign/missing game, one in
// neither source state, or one with no question at position (including a lost
// race) is ErrNotFound.
func (s *Store) OpenNextQuestion(ctx context.Context, gameID, organizerID string, position int32) (gen.Game, error) {
	game, err := s.q.OpenNextQuestion(ctx, gen.OpenNextQuestionParams{ID: gameID, OrganizerID: organizerID, Position: position})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}

// FinishGame transitions a game to finished from any of question_open,
// question_closed, revealed, or leaderboard; a foreign/missing game or one in
// another state (including a lost race) is ErrNotFound.
func (s *Store) FinishGame(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	game, err := s.q.FinishGame(ctx, gen.FinishGameParams{ID: gameID, OrganizerID: organizerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.Game{}, ErrNotFound
	}
	return game, err
}
