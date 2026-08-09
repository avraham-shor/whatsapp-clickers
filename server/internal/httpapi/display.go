package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// displaySettingsRequest mirrors the request body; the response is the
// full snapshot, like every other engine+hub route in this package.
// Pointer field and required: with a plain bool an omitted key decodes
// to false, and false is meaningful here - absence must be an explicit
// 400, never an accidental "turn animations back on" (the same reasoning
// handleUpdateScoring documents for its four pointers).
type displaySettingsRequest struct {
	ReducedMotion *bool `json:"reducedMotion"`
}

// handleUpdateDisplaySettings sets the room-level display settings and
// broadcasts the resulting snapshot (FR-9). PUT, not PATCH: it replaces
// the whole settings sub-resource, matching PUT /scoring.
//
// Broadcast before writing the REST response so an already-open display
// renders at least as promptly as the organizer's own dashboard - the
// same ordering handleOpenLobby documents.
func handleUpdateDisplaySettings(engine ControlEngine, hub SnapshotBroadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizerID, ok := requireOrganizer(w, r)
		if !ok {
			return
		}
		gameID, ok := gameIDParam(w, r)
		if !ok {
			return
		}
		var req displaySettingsRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.ReducedMotion == nil {
			writeValidationError(w, "reducedMotion is required")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		snapshot, err := engine.SetDisplaySettings(ctx, gameID, organizerID, *req.ReducedMotion)
		if err != nil {
			writeStoreError(w, err, "GAME_NOT_FOUND")
			return
		}
		hub.Broadcast(gameID, snapshot)
		slog.Info("display settings updated", "game_id", gameID, "organizer_id", organizerID, "reduced_motion", *req.ReducedMotion)
		writeJSON(w, http.StatusOK, snapshot)
	}
}
