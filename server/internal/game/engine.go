package game

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// ErrNotDraft means the game exists (and is owned by the caller) but is not
// in draft state — the primary, message-accurate rejection for OpenLobby.
var ErrNotDraft = errors.New("game: not in draft state")

// ErrNotLobby means the game exists but is not in lobby state — the
// rejection for StartGame.
var ErrNotLobby = errors.New("game: not in lobby state")

// ErrNoQuestions means the game is in lobby state but has no questions to
// start — the rejection for StartGame when the roster is empty.
var ErrNoQuestions = errors.New("game: has no questions")

// ErrNotQuestionOpen means the game exists but has no open question — the
// rejection for CloseQuestion.
var ErrNotQuestionOpen = errors.New("game: not in question_open state")

// ErrNotQuestionClosed means the game exists but its question is not
// closed — the rejection for Reveal.
var ErrNotQuestionClosed = errors.New("game: not in question_closed state")

// ErrNotRevealed means the game exists but its question is not revealed —
// the rejection for NextQuestion.
var ErrNotRevealed = errors.New("game: not in revealed state")

// ErrNotStoppable means the game exists but is not in a state a live round
// can be aborted from — the rejection for StopGame.
var ErrNotStoppable = errors.New("game: cannot be stopped from its current state")

// Store is the persistence surface the engine needs; *store.Store satisfies
// it. Consumer-defined here, not in store, per the dependency direction
// (game imports store, never the reverse).
type Store interface {
	GetGameForOrganizer(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	OpenGameLobby(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	ListParticipants(ctx context.Context, gameID string) ([]gen.Participant, error)
	GetGameByJoinCode(ctx context.Context, joinCode string) (gen.Game, error)
	CreateParticipant(ctx context.Context, gameID, phone, displayName, role string, allowedStates []string) (gen.Participant, bool, error)
	UpdateParticipantNameByPhone(ctx context.Context, phone, displayName string) (gen.Participant, error)
	ListQuestionsByGame(ctx context.Context, gameID, organizerID string) ([]gen.Question, error)
	StartGameFirstQuestion(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	CloseCurrentQuestion(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	RevealCurrentQuestion(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	OpenNextQuestion(ctx context.Context, gameID, organizerID string, position int32) (gen.Game, error)
	FinishGame(ctx context.Context, gameID, organizerID string) (gen.Game, error)
}

// Engine is the single write path for games.state (Enforcement Guidelines:
// "no component or handler mutates game state directly"). It builds every
// snapshot fresh from Postgres — nothing worth caching engine-side at this
// story's scale (one transition, no timers, no per-question state).
type Engine struct {
	store          Store
	platformNumber string
	logger         *slog.Logger
}

// NewEngine builds an Engine. platformNumber is the WhatsApp number shown
// on the lobby page (WHATSAPP_DISPLAY_NUMBER). logger may be nil, in which
// case slog.Default() is used.
func NewEngine(st Store, platformNumber string, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{store: st, platformNumber: platformNumber, logger: logger}
}

// OpenLobby transitions gameID from draft to lobby and returns the
// resulting snapshot. A non-draft game (including one that lost a
// concurrent transition race between the read and the write below) is
// ErrNotDraft; a missing/foreign game is store.ErrNotFound.
func (e *Engine) OpenLobby(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	if g.State != StateDraft {
		return Snapshot{}, ErrNotDraft
	}
	g, err = e.store.OpenGameLobby(ctx, gameID, organizerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// The race-guard WHERE clause (state = 'draft') lost a
			// concurrent race: the game still exists, it just stopped
			// being draft between the read above and this write —
			// ErrNotDraft, not ErrNotFound.
			return Snapshot{}, ErrNotDraft
		}
		return Snapshot{}, err
	}
	return e.snapshotAfterCommit(ctx, g), nil
}

// StartGame transitions gameID from lobby to question_open on its first
// question and returns the resulting snapshot. A non-lobby game (including
// one that lost a concurrent transition race) is ErrNotLobby; a lobby game
// with no questions is ErrNoQuestions; a missing/foreign game is
// store.ErrNotFound.
func (e *Engine) StartGame(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	if g.State != StateLobby {
		return Snapshot{}, ErrNotLobby
	}
	questions, err := e.store.ListQuestionsByGame(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	if len(questions) == 0 {
		return Snapshot{}, ErrNoQuestions
	}
	g, err = e.store.StartGameFirstQuestion(ctx, gameID, organizerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Same race-loss reinterpretation as OpenLobby: the guard
			// (state = 'lobby') lost the race between the read above and
			// this write.
			return Snapshot{}, ErrNotLobby
		}
		return Snapshot{}, err
	}
	return e.snapshotAfterCommit(ctx, g), nil
}

// CloseQuestion transitions gameID from question_open to question_closed
// and returns the resulting snapshot. A game not currently question_open
// (including one that lost a concurrent transition race) is
// ErrNotQuestionOpen; a missing/foreign game is store.ErrNotFound.
func (e *Engine) CloseQuestion(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	if g.State != StateQuestionOpen {
		return Snapshot{}, ErrNotQuestionOpen
	}
	g, err = e.store.CloseCurrentQuestion(ctx, gameID, organizerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Snapshot{}, ErrNotQuestionOpen
		}
		return Snapshot{}, err
	}
	return e.snapshotAfterCommit(ctx, g), nil
}

// Reveal transitions gameID from question_closed to revealed and returns
// the resulting snapshot. A game not currently question_closed (including
// one that lost a concurrent transition race) is ErrNotQuestionClosed; a
// missing/foreign game is store.ErrNotFound.
func (e *Engine) Reveal(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	if g.State != StateQuestionClosed {
		return Snapshot{}, ErrNotQuestionClosed
	}
	g, err = e.store.RevealCurrentQuestion(ctx, gameID, organizerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Snapshot{}, ErrNotQuestionClosed
		}
		return Snapshot{}, err
	}
	return e.snapshotAfterCommit(ctx, g), nil
}

// NextQuestion transitions gameID from revealed to question_open on the
// next question, or to finished when the revealed question was the last
// one — this is the "skip the Leaderboard" path (see story Dev Notes); this
// story never implements a control that enters the leaderboard state. A
// game not currently revealed (including one that lost a concurrent
// transition race) is ErrNotRevealed; a missing/foreign game is
// store.ErrNotFound.
func (e *Engine) NextQuestion(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	if g.State != StateRevealed {
		return Snapshot{}, ErrNotRevealed
	}
	questions, err := e.store.ListQuestionsByGame(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	nextPosition := g.CurrentQuestionPosition + 1
	hasNext := false
	for _, q := range questions {
		if q.Position == nextPosition {
			hasNext = true
			break
		}
	}
	if hasNext {
		g, err = e.store.OpenNextQuestion(ctx, gameID, organizerID, nextPosition)
	} else {
		g, err = e.store.FinishGame(ctx, gameID, organizerID)
	}
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Same race-loss reinterpretation as OpenLobby: the guard
			// (state = 'revealed') lost the race between the read above and
			// this write, whichever of the two writes above ran.
			return Snapshot{}, ErrNotRevealed
		}
		return Snapshot{}, err
	}
	return e.snapshotAfterCommit(ctx, g), nil
}

// StopGame aborts a live round by transitioning gameID to finished from
// question_open, question_closed, or revealed — not callable from draft,
// lobby, or finished (see story Dev Notes on why lobby-abandonment is out
// of scope). A game in a non-stoppable state (including one that lost a
// concurrent transition race) is ErrNotStoppable; a missing/foreign game is
// store.ErrNotFound.
func (e *Engine) StopGame(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	switch g.State {
	case StateQuestionOpen, StateQuestionClosed, StateRevealed:
	default:
		return Snapshot{}, ErrNotStoppable
	}
	g, err = e.store.FinishGame(ctx, gameID, organizerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Snapshot{}, ErrNotStoppable
		}
		return Snapshot{}, err
	}
	return e.snapshotAfterCommit(ctx, g), nil
}

// Snapshot returns the current live-state snapshot for gameID, scoped to
// organizerID (ownership + existence in one call). This is the read path
// ws.Handler calls on every new connection.
func (e *Engine) Snapshot(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	return e.buildSnapshot(ctx, g)
}

// snapshotAfterCommit builds the post-transition snapshot for g, whose
// state transition already committed. A failure building it here must not
// be reported as a failed transition — the caller would retry into a
// confusing 409 for a change that already happened, while every WS client
// stays stuck on the pre-transition snapshot. Degrades to an empty,
// questionless snapshot on failure and lets the next real read (a
// reconnect, or a future broadcast) pick up the true state.
//
// Detached from ctx's cancellation (context.WithoutCancel) and given its
// own bounded timeout instead: this build feeds the broadcast every other
// WS client receives, so the request that triggered the transition being
// aborted (a closed tab, a dropped connection) must not degrade that
// broadcast for everyone else — but an unbounded context would let a
// genuinely wedged DB hang here forever, so it still gets its own budget.
func (e *Engine) snapshotAfterCommit(ctx context.Context, g gen.Game) Snapshot {
	buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	snap, err := e.buildSnapshot(buildCtx, g)
	if err != nil {
		e.logger.Warn("post-transition snapshot build failed, degrading to an empty/questionless snapshot", "game_id", g.ID, "error", err)
		return emptySnapshot(e.platformNumber, g)
	}
	return snap
}

func (e *Engine) buildSnapshot(ctx context.Context, g gen.Game) (Snapshot, error) {
	participants, err := e.store.ListParticipants(ctx, g.ID)
	if err != nil {
		return Snapshot{}, err
	}
	// Non-nil so the wire always carries [], never null (newGameDetailPayload
	// precedent).
	summaries := make([]ParticipantSummary, 0, len(participants))
	for _, p := range participants {
		summaries = append(summaries, ParticipantSummary{ID: p.ID, DisplayName: p.DisplayName})
	}

	questions, err := e.store.ListQuestionsByGame(ctx, g.ID, g.OrganizerID)
	if err != nil {
		return Snapshot{}, err
	}
	var current *CurrentQuestion
	if g.CurrentQuestionPosition > 0 {
		for _, q := range questions {
			if q.Position == g.CurrentQuestionPosition {
				current = &CurrentQuestion{
					ID:               q.ID,
					Position:         int(q.Position),
					Type:             q.Type,
					Text:             q.Text,
					Options:          q.Options,
					TimeLimitSeconds: int(q.TimeLimitSeconds),
					AnswerCutoffAt:   g.AnswerCutoffAt.UTC().Format(time.RFC3339),
				}
				break
			}
		}
	}

	return Snapshot{
		GameID:           g.ID,
		State:            g.State,
		JoinCode:         g.JoinCode,
		PlatformNumber:   e.platformNumber,
		ParticipantCount: len(participants),
		Participants:     summaries,
		QuestionCount:    len(questions),
		CurrentQuestion:  current,
	}, nil
}

// emptySnapshot builds a snapshot with a non-nil, empty participant list and
// no question data — the degraded fallback for a post-commit snapshot build
// failure (snapshotAfterCommit).
func emptySnapshot(platformNumber string, g gen.Game) Snapshot {
	return Snapshot{
		GameID:           g.ID,
		State:            g.State,
		JoinCode:         g.JoinCode,
		PlatformNumber:   platformNumber,
		ParticipantCount: 0,
		Participants:     []ParticipantSummary{},
	}
}
