package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
)

// LobbyEngine is the state-transition surface handleOpenLobby needs;
// *game.Engine satisfies it.
type LobbyEngine interface {
	OpenLobby(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
}

// SnapshotBroadcaster is the fan-out surface handleOpenLobby needs;
// *ws.Hub satisfies it.
type SnapshotBroadcaster interface {
	Broadcast(gameID string, snapshot game.Snapshot)
}

func handleOpenLobby(engine LobbyEngine, hub SnapshotBroadcaster) http.HandlerFunc {
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
