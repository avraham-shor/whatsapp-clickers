// Package httpapi wires the HTTP surface: the JSON API under /api and
// SPA static serving for everything else. Handlers stay thin; data access
// goes through the store package.
package httpapi

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Pinger reports whether the database is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewRouter builds the full HTTP handler. static holds the built SPA
// (index.html at its root); nil disables static serving (API-only).
// webhook is mounted at /webhooks/whatsapp outside /api with no session
// middleware — Story 2.1's HMAC check is its own auth; nil omits the
// branch entirely (keeps callers that don't need it, e.g. most tests,
// compiling with a single added nil arg). engine and hub back the six
// game-control routes (open-lobby, start, close-question, reveal,
// next-question, stop) — either being nil omits all six (both are required
// together; a single argument can't observably desync them); wsHandler is
// mounted at /ws like webhook — nil omits that branch too. dispatcher is
// independently nilable — it only affects whether /start and /next-question
// also dispatch WhatsApp Question messages, never whether any route exists.
// resultDispatcher is independently nilable the same way — it only affects
// whether /reveal also dispatches WhatsApp personal-result messages, never
// whether any route exists. finalDispatcher is independently nilable the
// same way — it only affects whether /next-question and /stop also dispatch
// WhatsApp game-end messages, never whether any route exists.
func NewRouter(db Pinger, authSvc AuthService, games GameStore, static fs.FS, webhook http.Handler, engine ControlEngine, hub SnapshotBroadcaster, wsHandler http.Handler, dispatcher QuestionDispatcher, resultDispatcher ResultDispatcher, finalDispatcher FinalDispatcher) http.Handler {
	r := chi.NewRouter()

	if webhook != nil {
		r.Mount("/webhooks/whatsapp", webhook)
	}
	if wsHandler != nil {
		r.Get("/ws", wsHandler.ServeHTTP)
	}

	r.Route("/api", func(api chi.Router) {
		// The subrouter does not inherit the root handlers set below —
		// declare the JSON envelopes explicitly for the API branch.
		api.NotFound(func(w http.ResponseWriter, req *http.Request) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "no such endpoint")
		})
		api.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed for this endpoint")
		})

		// Public: health (Railway monitoring) and login itself.
		api.Get("/health", handleHealth(db))
		api.Post("/auth/login", handleLogin(authSvc))

		// Everything else on the API branch requires a live session.
		api.Group(func(protected chi.Router) {
			protected.Use(RequireOrganizer(authSvc))
			protected.Post("/auth/logout", handleLogout(authSvc))
			protected.Get("/auth/me", handleMe())

			// The Question Bank is first-party and identical for every
			// organizer — open with the session guard, no ownership scoping.
			protected.Get("/question-packages", handleListQuestionPackages(games))

			protected.Route("/games", func(g chi.Router) {
				g.Post("/", handleCreateGame(games))
				g.Get("/", handleListGames(games))
				g.Route("/{gameID}", func(gr chi.Router) {
					gr.Get("/", handleGetGame(games))
					// Outside the engine/hub guard deliberately: the
					// post-game summary is a pure store read like GET /
					// and PUT /scoring, and gating it on the engine would
					// make it vanish in every test router passing nil.
					gr.Get("/results", handleGameResults(games))
					gr.Put("/scoring", handleUpdateScoring(games))
					if engine != nil && hub != nil {
						gr.Post("/open-lobby", handleOpenLobby(engine, hub))
						gr.Post("/start", handleStartGame(engine, hub, dispatcher))
						gr.Post("/close-question", handleCloseQuestion(engine, hub))
						gr.Post("/reveal", handleReveal(engine, hub, resultDispatcher))
						gr.Post("/show-leaderboard", handleShowLeaderboard(engine, hub))
						gr.Post("/next-question", handleNextQuestion(engine, hub, dispatcher, finalDispatcher))
						gr.Post("/stop", handleStopGame(engine, hub, finalDispatcher))
						// Inside the guard deliberately, unlike /results
						// above: this route genuinely needs both the
						// engine and the hub, so a test router passing
						// nil for either must not expose it.
						gr.Put("/display-settings", handleUpdateDisplaySettings(engine, hub))
					}
					gr.Route("/questions", func(qr chi.Router) {
						qr.Post("/", handleCreateQuestion(games))
						qr.Post("/reorder", handleReorderQuestions(games))
						qr.Post("/import-package", handleImportPackage(games))
						qr.Put("/{questionID}", handleUpdateQuestion(games))
						qr.Delete("/{questionID}", handleDeleteQuestion(games))
					})
				})
			})
		})
	})

	spa := spaHandler(static)
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if isReservedPath(req.URL.Path) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "no such endpoint")
			return
		}
		spa(w, req)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed for this endpoint")
	})

	return r
}

// isReservedPath reports whether the path belongs to the API surface and
// must never fall back to the SPA.
func isReservedPath(path string) bool {
	for _, prefix := range []string{"/api", "/ws", "/webhooks"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func handleHealth(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Bound server-side so a black-holed DB yields a prompt 503
		// instead of hanging until the client gives up.
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database is unreachable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "ok"})
	}
}

// spaHandler serves files from static, falling back to index.html for
// client-side routes (e.g. /display/:gameId).
func spaHandler(static fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if static == nil {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if f, err := static.Open(name); err == nil {
			f.Close()
			http.FileServerFS(static).ServeHTTP(w, r)
			return
		}
		// A missing file-like path (e.g. a stale hashed bundle after a
		// redeploy) must 404, not serve index.html as if it were a route.
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, static, "index.html")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}
