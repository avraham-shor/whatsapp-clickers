package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
	"github.com/avraham-shor/whatsapp-clickers/internal/game"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
)

// sessionCookieName mirrors httpapi's cookie name (hardcoded, not imported —
// ws must not import httpapi; that would invert the dependency direction).
const sessionCookieName = "wc_session"

// Authenticator is the session-authentication surface Handler needs;
// *auth.Service satisfies it directly (same signature httpapi.AuthService
// uses) — importing internal/auth here is fine, it's foundational, like
// httpapi's own dependency on it.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (auth.Organizer, error)
}

// SnapshotReader is the read path Handler calls on every new connection;
// *game.Engine satisfies it.
type SnapshotReader interface {
	Snapshot(ctx context.Context, gameID, organizerID string) (game.Snapshot, error)
}

// NewHandler builds the /ws HTTP handler. Every rejection (bad/missing
// session, wrong role, malformed/foreign/missing game) answers before the
// WebSocket handshake — a rejected request never upgrades.
func NewHandler(authSvc Authenticator, engine SnapshotReader, hub *Hub, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "valid session required")
			return
		}
		authCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		organizer, err := authSvc.Authenticate(authCtx, cookie.Value)
		cancel()
		if err != nil {
			if errors.Is(err, auth.ErrNoSession) {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "valid session required")
				return
			}
			writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database is unreachable")
			return
		}

		role := r.URL.Query().Get("role")
		if role != "host" && role != "display" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "role must be host or display")
			return
		}

		gameID := r.URL.Query().Get("gameId")
		if !isUUID(gameID) {
			// Malformed is indistinguishable from missing (same
			// non-enumeration posture as the REST 404s) — never a 503 from
			// pgx choking on non-UUID text.
			writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "no such game for this organizer")
			return
		}

		// Captured before the snapshot read below so a Broadcast landing
		// during that read (or during Accept/register, just after) is
		// detectable — see the comment above hub.register a few lines down.
		seqBeforeBuild := hub.currentSeq(gameID)

		snapCtx, snapCancel := context.WithTimeout(r.Context(), 5*time.Second)
		snapshot, err := engine.Snapshot(snapCtx, gameID, organizer.ID)
		snapCancel()
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "GAME_NOT_FOUND", "no such game for this organizer")
				return
			}
			logger.Error("ws snapshot lookup failed", "error", err)
			writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database is unreachable")
			return
		}

		sock, err := websocket.Accept(w, r, nil)
		if err != nil {
			logger.Warn("ws accept failed", "game_id", gameID, "error", err)
			return
		}
		defer sock.CloseNow()

		// Registering and enqueuing the initial snapshot through the same
		// channel as later broadcasts means the two can never race for
		// this connection. A narrower race remains: a Broadcast can land
		// between the engine.Snapshot() read above and this register call,
		// which would otherwise leave the just-built snapshot stale while
		// still being stamped with the hub’s now-current seq (defeating the
		// client’s stale-drop guard, which can never flag a frame claiming
		// to be current). Detect it by comparing the hub’s seq before and
		// after register: if it moved, refetch fresh rather than shipping
		// stale state under a current-looking seq.
		c := hub.register(r.Context(), gameID, sock)
		seq := seqBeforeBuild
		if seqAfterRegister := hub.currentSeq(gameID); seqAfterRegister != seqBeforeBuild {
			refreshCtx, refreshCancel := context.WithTimeout(r.Context(), 5*time.Second)
			fresh, ferr := engine.Snapshot(refreshCtx, gameID, organizer.ID)
			refreshCancel()
			if ferr == nil {
				snapshot = fresh
				seq = seqAfterRegister
			} else {
				// Rare double-fault: keep the pre-register snapshot, but keep
				// its seq paired with it too (seqBeforeBuild) rather than
				// mismatching stale content with a current-looking seq — the
				// next broadcast or reconnect still heals it.
				logger.Warn("post-register snapshot refresh failed, sending pre-register snapshot", "game_id", gameID, "error", ferr)
			}
		}

		initial, err := json.Marshal(envelope{Type: "snapshot", Seq: seq, State: snapshot})
		if err != nil {
			// The connection is already upgraded at this point — an HTTP
			// error response is no longer possible. Snapshot's fields are all
			// plain strings/ints/slices (see hub.go's identical
			// Marshal-cannot-fail note); this is a defensive backstop, not an
			// expected path.
			logger.Error("marshal initial snapshot failed", "game_id", gameID, "error", err)
			return
		}
		if !c.enqueue(initial) {
			logger.Warn("outbound socket queue full, initial snapshot dropped", "game_id", gameID)
		}
		logger.Info("ws connected", "game_id", gameID, "organizer_id", organizer.ID, "role", role)

		// The server never expects client-sent application messages
		// (organizer actions are REST-only) but coder/websocket requires
		// draining reads to detect the client closing and to service
		// control frames.
		for {
			if _, _, err := sock.Read(r.Context()); err != nil {
				break
			}
		}
		hub.unregister(gameID, c)
		logger.Info("ws disconnected", "game_id", gameID, "organizer_id", organizer.ID)
	})
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError mirrors httpapi's JSON error envelope, duplicated locally
// (~10 lines) rather than importing httpapi, which would invert the
// dependency direction.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorBody{Error: errorDetail{Code: code, Message: message}})
}

// isUUID mirrors httpapi.isUUID, duplicated locally for the same reason as
// writeError above.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return false
		}
	}
	return true
}
