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
	Reveal(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
	NextQuestion(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
	StopGame(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
}

// SnapshotBroadcaster is the fan-out surface handleOpenLobby needs;
// *ws.Hub satisfies it.
type SnapshotBroadcaster interface {
	Broadcast(gameID string, snapshot game.Snapshot)
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

func handleStartGame(engine ControlEngine, hub SnapshotBroadcaster) http.HandlerFunc {
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

func handleReveal(engine ControlEngine, hub SnapshotBroadcaster) http.HandlerFunc {
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
		snapshot, err := engine.Reveal(ctx, gameID, organizerID)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		hub.Broadcast(gameID, snapshot)
		slog.Info("question revealed", "game_id", gameID, "organizer_id", organizerID)
		writeJSON(w, http.StatusOK, snapshot)
	}
}

func handleNextQuestion(engine ControlEngine, hub SnapshotBroadcaster) http.HandlerFunc {
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
