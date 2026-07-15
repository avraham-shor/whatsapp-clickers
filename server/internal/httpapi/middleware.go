package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
)

// sessionCookieName is the HTTP-only cookie carrying the raw session token.
const sessionCookieName = "wc_session"

type contextKey struct{ name string }

var organizerKey = &contextKey{"organizer"}

// OrganizerFromContext returns the organizer injected by RequireOrganizer;
// ok is false when the request never passed the middleware.
func OrganizerFromContext(ctx context.Context) (auth.Organizer, bool) {
	org, ok := ctx.Value(organizerKey).(auth.Organizer)
	return org, ok
}

// RequireOrganizer rejects requests without a live session (401 envelope)
// and injects the authenticated organizer into the request context.
func RequireOrganizer(svc AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "valid session required")
				return
			}
			// Bound the session lookup like every other handler-side DB call.
			authCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			org, err := svc.Authenticate(authCtx, cookie.Value)
			cancel()
			if err != nil {
				if errors.Is(err, auth.ErrNoSession) {
					writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "valid session required")
					return
				}
				writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database is unreachable")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), organizerKey, org)))
		})
	}
}
