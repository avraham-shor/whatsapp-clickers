package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
)

// ControlEngine is the state-transition surface the lobby/live handlers
// need; *game.Engine satisfies it.
type ControlEngine interface {
	OpenLobby(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
	StartGame(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
	CloseQuestion(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
	Reveal(ctx context.Context, gameID, organizerID string) (game.Snapshot, int32, error)
	NextQuestion(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
	StopGame(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
	PlayerRecipients(ctx context.Context, gameID string) ([]string, error)
	ResultsForRevealedQuestion(ctx context.Context, gameID, organizerID string, position int32) (game.RevealedQuestionResults, error)
}

// SnapshotBroadcaster is the fan-out surface handleOpenLobby needs;
// *ws.Hub satisfies it.
type SnapshotBroadcaster interface {
	Broadcast(gameID string, snapshot game.Snapshot)
}

// QuestionDispatcher is the WhatsApp fan-out surface handleStartGame/
// handleNextQuestion need; *wa.QuestionNotifier satisfies it. Consumer-
// defined here, referencing only game types, so httpapi never imports wa
// (main.go is the sole place a concrete *wa.QuestionNotifier is wired in).
type QuestionDispatcher interface {
	DispatchQuestionOpened(gameID string, question game.CurrentQuestion, questionCount int, recipients []string)
}

// ResultDispatcher is the WhatsApp fan-out surface handleReveal needs;
// *wa.ResultNotifier satisfies it. Consumer-defined here, referencing
// only game types, so httpapi never imports wa — same posture as
// QuestionDispatcher.
type ResultDispatcher interface {
	DispatchAnswerRevealed(gameID string, results []game.PersonalResult, correctAnswerText string, isLastQuestion bool)
}

// dispatchQuestionOpened hands the WhatsApp question-opened burst to
// dispatcher when snapshot reflects a freshly opened question. The caller
// runs this in its own goroutine after writing the HTTP response (see
// handleStartGame/handleNextQuestion) — PlayerRecipients is a real DB round
// trip, and gating the organizer-facing response on it would mean an
// organizer's "start"/"next question" click could visibly stall for up to
// this function's own timeout (code review, story 3.2, 2026-08-04).
//
// Guards on snapshot.State == game.StateQuestionOpen rather than a bare nil
// CurrentQuestion check: buildSnapshot populates CurrentQuestion whenever
// CurrentQuestionPosition > 0 regardless of state, so a nil-only check
// cannot distinguish "NextQuestion finished the game" (State == finished,
// correctly no dispatch) from "the transition opened a question but
// snapshotAfterCommit degraded to an emptySnapshot on a build failure"
// (State == question_open, CurrentQuestion nil — and this WAS a case that
// needed dispatch). Both silent-skip paths below (a degraded snapshot, and
// a PlayerRecipients failure) log at ERROR — the transition already
// committed, so the failure is never reported to the organizer as a failed
// request, but it must not vanish from the logs either.
//
// Detached from ctx's cancellation with its own bounded timeout — same
// reasoning as engine.snapshotAfterCommit (story 3.1): the transition
// already committed and its WS snapshot already broadcast by the time
// this runs, so the organizer's connection closing must not skip
// delivering the question to everyone else.
func dispatchQuestionOpened(ctx context.Context, engine ControlEngine, dispatcher QuestionDispatcher, gameID string, snapshot game.Snapshot) {
	if dispatcher == nil || snapshot.State != game.StateQuestionOpen {
		return
	}
	if snapshot.CurrentQuestion == nil {
		slog.Error("question dispatch skipped, snapshot missing current question for a question_open state", "game_id", gameID)
		return
	}
	dispatchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	recipients, err := engine.PlayerRecipients(dispatchCtx, gameID)
	if err != nil {
		slog.Error("question dispatch skipped, could not list player recipients", "game_id", gameID, "error", err)
		return
	}
	dispatcher.DispatchQuestionOpened(gameID, *snapshot.CurrentQuestion, snapshot.QuestionCount, recipients)
}

// dispatchAnswerRevealed hands the WhatsApp personal-result burst to
// dispatcher when snapshot reflects a freshly revealed question. Spawned
// as a goroutine after the HTTP response is written (see handleReveal)
// — ResultsForRevealedQuestion is a real DB round trip (question lookup
// + leaderboard + per-answer read), and gating the organizer-facing
// response on it would mean an organizer's "reveal" click could
// visibly stall for up to this function's own timeout (same reasoning
// as dispatchQuestionOpened, story 3.2).
//
// position is Reveal's own second return value — the committed game
// row's current_question_position — NOT snapshot.CurrentQuestion.Position.
// This matters precisely when the snapshot is degraded: snapshotAfterCommit
// falls back to emptySnapshot on any buildSnapshot failure, and
// emptySnapshot carries State = "revealed" with a nil CurrentQuestion. A
// version of this function that read the position off the snapshot would
// pass the state guard, find nil, and silently drop every answering
// participant's result — permanently, since Reveal refuses to run again
// from the revealed state, so there is no retry. Binding to Reveal's own
// committed position keeps the dispatch alive through a degraded snapshot
// while preserving the story's original race guarantee: it is still the
// specific position the caller saw revealed, never re-derived by a fresh
// read that a concurrent NextQuestion could have advanced (code review,
// story 3.8).
//
// Guards on snapshot.State == game.StateRevealed as the "a question was
// just revealed" signal, and on a positive position — position is 0 only
// on Reveal's error paths, which the caller has already returned on.
//
// Detached from ctx's cancellation with its own bounded timeout — same
// reasoning as dispatchQuestionOpened/engine.snapshotAfterCommit: the
// transition already committed and its WS snapshot already broadcast
// by the time this runs, so the organizer's connection closing must
// not skip delivering results to everyone who answered.
func dispatchAnswerRevealed(ctx context.Context, engine ControlEngine, dispatcher ResultDispatcher, gameID, organizerID string, snapshot game.Snapshot, position int32) {
	if dispatcher == nil || snapshot.State != game.StateRevealed {
		return
	}
	if position < 1 {
		slog.Error("result dispatch skipped, revealed position is not set", "game_id", gameID, "position", position)
		return
	}
	dispatchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	results, err := engine.ResultsForRevealedQuestion(dispatchCtx, gameID, organizerID, position)
	if err != nil {
		slog.Error("result dispatch skipped, could not resolve revealed question results", "game_id", gameID, "error", err)
		return
	}
	dispatcher.DispatchAnswerRevealed(gameID, results.Results, results.CorrectAnswer, results.IsLastQuestion)
}

func handleOpenLobby(engine ControlEngine, hub SnapshotBroadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snapshot, err := engine.OpenLobby(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		// Broadcast before writing the REST response, so any already-open
		// /ws client renders at least as promptly as the REST caller.
		hub.Broadcast(gameID, snapshot)
		slog.Info("game lobby opened", "game_id", gameID, "organizer_id", organizerID)
		writeJSON(w, http.StatusOK, snapshot)
	}
}

func handleStartGame(engine ControlEngine, hub SnapshotBroadcaster, dispatcher QuestionDispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snapshot, err := engine.StartGame(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		hub.Broadcast(gameID, snapshot)
		slog.Info("game started", "game_id", gameID, "organizer_id", organizerID)
		writeJSON(w, http.StatusOK, snapshot)
		go dispatchQuestionOpened(ctx, engine, dispatcher, gameID, snapshot)
	}
}

func handleCloseQuestion(engine ControlEngine, hub SnapshotBroadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snapshot, err := engine.CloseQuestion(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		hub.Broadcast(gameID, snapshot)
		slog.Info("question closed", "game_id", gameID, "organizer_id", organizerID)
		writeJSON(w, http.StatusOK, snapshot)
	}
}

func handleReveal(engine ControlEngine, hub SnapshotBroadcaster, dispatcher ResultDispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snapshot, position, err := engine.Reveal(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		hub.Broadcast(gameID, snapshot)
		slog.Info("question revealed", "game_id", gameID, "organizer_id", organizerID, "position", position)
		writeJSON(w, http.StatusOK, snapshot)
		go dispatchAnswerRevealed(ctx, engine, dispatcher, gameID, organizerID, snapshot, position)
	}
}

func handleNextQuestion(engine ControlEngine, hub SnapshotBroadcaster, dispatcher QuestionDispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snapshot, err := engine.NextQuestion(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		hub.Broadcast(gameID, snapshot)
		slog.Info("game advanced", "game_id", gameID, "organizer_id", organizerID, "state", snapshot.State)
		writeJSON(w, http.StatusOK, snapshot)
		go dispatchQuestionOpened(ctx, engine, dispatcher, gameID, snapshot)
	}
}

func handleStopGame(engine ControlEngine, hub SnapshotBroadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snapshot, err := engine.StopGame(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		hub.Broadcast(gameID, snapshot)
		slog.Info("game stopped", "game_id", gameID, "organizer_id", organizerID)
		writeJSON(w, http.StatusOK, snapshot)
	}
}
