package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/avraham-shor/whatsapp-clickers/internal/game"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
)

// maxMutationBodyBytes caps every authenticated mutation body (1.2 hardening
// pattern): generous for question payloads, hostile to junk uploads.
const maxMutationBodyBytes = 64 << 10

// decodeJSON reads a size-capped JSON body into dst, emitting the 413/400
// envelope itself on failure; a false return means the response is written.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxMutationBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body is too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "body must be valid JSON matching the endpoint's shape")
		return false
	}
	return true
}

// writeValidationError reports a field-level rule violation; the message
// names the offending field (developer-facing English).
func writeValidationError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, "VALIDATION_FAILED", message)
}

// writeStoreError is the central domain-error → envelope mapper (deferred
// from 1.2 to this story's richer error surface). notFoundCode carries the
// resource flavor: GAME_NOT_FOUND or QUESTION_NOT_FOUND — existence must not
// leak, so ownership failures wear the same 404 as true misses.
func writeStoreError(w http.ResponseWriter, err error, notFoundCode string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, notFoundCode, "no such resource for this organizer")
	case errors.Is(err, store.ErrReorderMismatch):
		writeValidationError(w, "questionIds must be an exact permutation of the game's question ids")
	case errors.Is(err, game.ErrNotDraft):
		writeError(w, http.StatusConflict, "GAME_NOT_EDITABLE", "game must be in draft state to open its lobby")
	case errors.Is(err, game.ErrNotLobby):
		writeError(w, http.StatusConflict, "GAME_NOT_LOBBY", "game must be in lobby state to start")
	case errors.Is(err, game.ErrNoQuestions):
		writeError(w, http.StatusConflict, "GAME_NO_QUESTIONS", "game must have at least one question to start")
	case errors.Is(err, game.ErrNotQuestionOpen):
		writeError(w, http.StatusConflict, "GAME_NOT_QUESTION_OPEN", "game must have an open question to close")
	case errors.Is(err, game.ErrNotQuestionClosed):
		writeError(w, http.StatusConflict, "GAME_NOT_QUESTION_CLOSED", "question must be closed before it can be revealed")
	case errors.Is(err, game.ErrGradingIncomplete):
		writeError(w, http.StatusConflict, "GRADING_INCOMPLETE", "not every received answer is graded yet")
	case errors.Is(err, game.ErrNotRevealed):
		writeError(w, http.StatusConflict, "GAME_NOT_REVEALED", "question must be revealed before advancing")
	case errors.Is(err, game.ErrNotStoppable):
		writeError(w, http.StatusConflict, "GAME_NOT_STOPPABLE", "game cannot be stopped from its current state")
	default:
		// Infrastructure: the wire code alone must not be the only triage
		// signal — record the underlying cause.
		slog.Error("store call failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database is unreachable")
	}
}

// isUUID reports whether s is a canonically formatted UUID. Malformed path
// or body IDs short-circuit to their domain response instead of surfacing a
// database text-parse error as a 503.
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
