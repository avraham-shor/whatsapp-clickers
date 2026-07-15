package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
)

// AuthService is the credential/session surface handlers depend on;
// *auth.Service satisfies it.
type AuthService interface {
	Login(ctx context.Context, username, password string) (string, auth.Organizer, error)
	Authenticate(ctx context.Context, token string) (auth.Organizer, error)
	Logout(ctx context.Context, token string) error
}

// organizerPayload is the wire shape of a signed-in organizer (camelCase,
// direct payload — no wrapper).
type organizerPayload struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// sessionCookie builds the wc_session cookie; maxAge < 0 emits Max-Age=0,
// clearing it with otherwise identical attributes.
func sessionCookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

// maxLoginBodyBytes bounds the unauthenticated login payload — generous for
// credentials JSON, hostile to junk uploads.
const maxLoginBodyBytes = 4 << 10

func handleLogin(svc AuthService) http.HandlerFunc {
	type loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body is too large")
				return
			}
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "body must be JSON with username and password")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		token, org, err := svc.Login(ctx, req.Username, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrInvalidCredentials):
				// Log the attempted username only — never passwords or tokens.
				slog.Info("login failed", "username", req.Username)
				writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "username and password do not match")
			case errors.Is(err, auth.ErrTokenGeneration):
				slog.Error("login token generation failed", "error", err)
				writeError(w, http.StatusInternalServerError, "INTERNAL", "could not create a session")
			default:
				// DB failure or timeout — the wire code alone must not be
				// the only triage signal, so record the underlying cause.
				slog.Error("login infrastructure failure", "error", err)
				writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database is unreachable")
			}
			return
		}
		slog.Info("login succeeded", "organizer_id", org.ID, "username", org.Username)
		http.SetCookie(w, sessionCookie(token, int(auth.SessionTTL/time.Second)))
		writeJSON(w, http.StatusOK, organizerPayload{ID: org.ID, Username: org.Username})
	}
}

func handleLogout(svc AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// RequireOrganizer guarantees the cookie exists; guard anyway.
		if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			if err := svc.Logout(ctx, cookie.Value); err != nil {
				// The browser asked to log out — honor that locally even
				// though the server-side delete failed, so the cookie never
				// outlives the user's intent.
				slog.Error("logout failed server-side", "error", err)
				http.SetCookie(w, sessionCookie("", -1))
				writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database is unreachable")
				return
			}
		}
		if org, ok := OrganizerFromContext(r.Context()); ok {
			slog.Info("logout", "organizer_id", org.ID, "username", org.Username)
		}
		http.SetCookie(w, sessionCookie("", -1))
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		org, ok := OrganizerFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "valid session required")
			return
		}
		writeJSON(w, http.StatusOK, organizerPayload{ID: org.ID, Username: org.Username})
	}
}
