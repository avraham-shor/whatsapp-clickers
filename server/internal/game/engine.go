package game

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/grading"
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

// ErrGradingIncomplete means the current question's answers aren't all
// graded yet — the rejection for Reveal when grading is still in
// flight (FR-16 epic AC-3). A no-op condition in this story (MCQ/Exact
// grading is synchronous); load-bearing from Story 3.6's async AI
// stage onward.
var ErrGradingIncomplete = errors.New("game: grading not yet complete for current question")

// Store is the persistence surface the engine needs; *store.Store satisfies
// it. Consumer-defined here, not in store, per the dependency direction
// (game imports store, never the reverse).
type Store interface {
	GetGameForOrganizer(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	OpenGameLobby(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	ListParticipants(ctx context.Context, gameID string) ([]gen.Participant, error)
	GetGameByJoinCode(ctx context.Context, joinCode string) (gen.Game, error)
	GetGameByID(ctx context.Context, gameID string) (gen.Game, error)
	CreateParticipant(ctx context.Context, gameID, phone, displayName, role string, allowedStates []string) (gen.Participant, bool, error)
	UpdateParticipantNameByPhone(ctx context.Context, phone, displayName string) (gen.Participant, error)
	ListQuestionsByGame(ctx context.Context, gameID, organizerID string) ([]gen.Question, error)
	StartGameFirstQuestion(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	CloseCurrentQuestion(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	ListAnswersForScoring(ctx context.Context, gameID string, position int32) ([]store.AnswerForScoring, error)
	RevealCurrentQuestionAndAwardPoints(ctx context.Context, gameID, organizerID string, points []store.AnswerPointsParams) (gen.Game, error)
	GetLeaderboard(ctx context.Context, gameID string) ([]store.ParticipantScore, error)
	OpenNextQuestion(ctx context.Context, gameID, organizerID string, position int32) (gen.Game, error)
	FinishGame(ctx context.Context, gameID, organizerID string) (gen.Game, error)
	GetOpenQuestionForPlayer(ctx context.Context, phone string) (gen.GetOpenQuestionForPlayerRow, error)
	RecordAnswer(ctx context.Context, arg store.RecordAnswerParams) (gen.Answer, error)
	CountAnswersByQuestion(ctx context.Context, questionID string) (int64, error)
	CountUngradedAnswersForCurrentQuestion(ctx context.Context, gameID string) (int64, error)
	UpdateAnswerGrade(ctx context.Context, answerID string, isCorrect bool, stage string) error
}

// Engine is the single write path for games.state (Enforcement Guidelines:
// "no component or handler mutates game state directly"). It builds every
// snapshot fresh from Postgres — nothing worth caching engine-side at this
// story's scale (one transition, no timers, no per-question state).
type Engine struct {
	store          Store
	platformNumber string
	logger         *slog.Logger
	aiGrader       grading.AIGrader
	runAsync       func(func())
	// gradeSem bounds how many AI Semantic stage calls run at once, and
	// gradeWG tracks them so a shutdown can wait for their verdicts to land
	// instead of abandoning pending rows. See WithMaxConcurrentAIGrades and
	// WaitForGrading.
	gradeSem chan struct{}
	gradeWG  sync.WaitGroup
}

// EngineOption configures optional Engine dependencies.
type EngineOption func(*Engine)

// WithAIGrader sets the AI Semantic stage's grader (Story 3.6). Callers
// that omit it get a nil aiGrader: a free_text answer that misses both
// Exact and Fuzzy is then persisted pending and logged at WARN, and only
// the orphan sweep will ever resolve it — a misconfiguration, not a mode.
// Production wiring (main.go) always supplies one.
func WithAIGrader(g grading.AIGrader) EngineOption {
	return func(e *Engine) { e.aiGrader = g }
}

// WithMaxConcurrentAIGrades caps how many AI Semantic stage calls are in
// flight at once (Story 3.6). n <= 0 is ignored.
//
// Unbounded fan-out was the original shape: one goroutine, one Anthropic
// call and one deadline-free pool acquisition per double-miss answer. A
// large room on a hard free-text question would then fire a hundred
// simultaneous Opus calls — rate-limit rejections eating the 5s per-call
// budget in backoff until every one of them fails closed to *incorrect* —
// while a hundred waiters starved a pgxpool sized max(4, numCPU), stalling
// the webhook handlers still recording answers. Bounded like every other
// outbound-I/O path here (wa.Dispatcher's worker pool). Code review
// finding, story 3.6.
func WithMaxConcurrentAIGrades(n int) EngineOption {
	return func(e *Engine) {
		if n > 0 {
			e.gradeSem = make(chan struct{}, n)
		}
	}
}

// WithAsyncRunner overrides how RecordAnswer launches AI grading —
// production defaults to a real goroutine; tests substitute a
// synchronous runner so the pending→graded transition is deterministic
// (no time.Sleep polling, matching this codebase's "deterministic tests
// only" testing standard — see deferred-work.md's 2.1 rate-limiter entry
// for why that standard exists).
func WithAsyncRunner(run func(func())) EngineOption {
	return func(e *Engine) { e.runAsync = run }
}

// NewEngine builds an Engine. platformNumber is the WhatsApp number shown
// on the lobby page (WHATSAPP_DISPLAY_NUMBER). logger may be nil, in which
// case slog.Default() is used. opts is a variadic functional-options tail
// (Story 3.6's WithAIGrader/WithAsyncRunner) rather than new positional
// parameters — this keeps every existing 3-arg NewEngine(...) call
// compiling unchanged (production and every pre-3.6 test).
func NewEngine(st Store, platformNumber string, logger *slog.Logger, opts ...EngineOption) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	e := &Engine{
		store:          st,
		platformNumber: platformNumber,
		logger:         logger,
		runAsync:       func(f func()) { go f() },
		gradeSem:       make(chan struct{}, DefaultMaxConcurrentAIGrades),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// WaitForGrading blocks until every in-flight AI grading goroutine has
// persisted its verdict, or until ctx is done — whichever comes first. It
// reports whether grading drained cleanly.
//
// Called during shutdown, after the HTTP server has stopped (so no new
// answers can arrive) and before the pool closes. Without it, SIGTERM
// abandons every in-flight verdict: those rows stay stage IS NULL, and the
// orphan sweep that would eventually rescue them runs in a *different*
// process which, in a zero-downtime redeploy, has already booted and swept
// before this one dies. Draining here is what keeps that the rare case
// rather than the normal one. Code review finding, story 3.6.
func (e *Engine) WaitForGrading(ctx context.Context) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.gradeWG.Wait()
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
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
// missing/foreign game is store.ErrNotFound; a question_closed game with
// outstanding ungraded answers is ErrGradingIncomplete (FR-16 epic AC-3).
// It also computes and persists Speed Bonus scoring for the revealed
// question (FR-17, story 3.7).
func (e *Engine) Reveal(ctx context.Context, gameID, organizerID string) (Snapshot, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return Snapshot{}, err
	}
	if g.State != StateQuestionClosed {
		return Snapshot{}, ErrNotQuestionClosed
	}
	outstanding, err := e.store.CountUngradedAnswersForCurrentQuestion(ctx, gameID)
	if err != nil {
		return Snapshot{}, err
	}
	if outstanding > 0 {
		return Snapshot{}, ErrGradingIncomplete
	}

	answers, err := e.store.ListAnswersForScoring(ctx, gameID, g.CurrentQuestionPosition)
	if err != nil {
		return Snapshot{}, err
	}
	cfg := ScoringConfig{
		PointsPerCorrect: g.PointsPerCorrect,
		SpeedBonusFirst:  g.SpeedBonusFirst,
		SpeedBonusSecond: g.SpeedBonusSecond,
		SpeedBonusThird:  g.SpeedBonusThird,
	}
	points := AwardPoints(answers, cfg)

	g, err = e.store.RevealCurrentQuestionAndAwardPoints(ctx, gameID, organizerID, points)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Race-loss reinterpretation, same as every other transition
			// here. Only the initial RevealCurrentQuestion UPDATE inside
			// the transaction can return zero rows this way — the batch
			// points write is an :exec whose targets were resolved moments
			// ago by this same request, so an error from it propagates
			// here as a genuine, un-reinterpreted failure instead.
			//
			// But WHICH condition that UPDATE missed on is still ambiguous,
			// and this remains a known gap rather than a solved problem.
			// Unlike every other transition in this file, its guard is not
			// single-condition (queries/games.sql): it can miss because a
			// concurrent transition moved the game off question_closed,
			// because no questions row matches current_question_position
			// (the UPDATE ... FROM join is inner), or because an answer for
			// the current question is ungraded (the NOT EXISTS clause). All
			// three collapse to ErrNotQuestionClosed, so the cause reported
			// to the operator can be wrong.
			//
			// The count above cannot go stale underneath us: an answer can
			// only be inserted while the game is question_open
			// (RecordAnswer's own guard), and Reveal runs only from
			// question_closed. Story 3.6 made the grading condition
			// genuinely reachable (async AI verdicts), which is when the
			// original version of this comment said these deserve distinct
			// errors rather than one message that can send an operator
			// after the wrong problem — that is still true and still not
			// done. Deferred at 3.7's code review; revisit whenever this
			// path next changes.
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

// PlayerRecipients returns the phone numbers of every player-role
// Participant in gameID — the WhatsApp Question-dispatch recipient list
// (FR-4); Spectators are excluded. Non-nil, possibly-empty slice, same
// "never null on the wire" discipline as buildSnapshot's participant
// summaries, even though this return value is never serialized. Unscoped by
// organizerID: every call site has already validated ownership via the
// state transition that immediately preceded it — identical trust posture
// to buildSnapshot's own unscoped ListParticipants call.
func (e *Engine) PlayerRecipients(ctx context.Context, gameID string) ([]string, error) {
	participants, err := e.store.ListParticipants(ctx, gameID)
	if err != nil {
		return nil, err
	}
	phones := make([]string, 0, len(participants))
	for _, p := range participants {
		if p.Role == RolePlayer {
			phones = append(phones, p.Phone)
		}
	}
	return phones, nil
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
				count, err := e.store.CountAnswersByQuestion(ctx, current.ID)
				if err != nil {
					return Snapshot{}, err
				}
				current.AnsweredCount = int(count)
				break
			}
		}
	}

	scores, err := e.store.GetLeaderboard(ctx, g.ID)
	if err != nil {
		return Snapshot{}, err
	}
	leaderboard := RankLeaderboard(scores)

	return Snapshot{
		GameID:           g.ID,
		State:            g.State,
		JoinCode:         g.JoinCode,
		PlatformNumber:   e.platformNumber,
		ParticipantCount: len(participants),
		Participants:     summaries,
		QuestionCount:    len(questions),
		CurrentQuestion:  current,
		Leaderboard:      leaderboard,
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
		Leaderboard:      []LeaderboardEntry{},
	}
}
