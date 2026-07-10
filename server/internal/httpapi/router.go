// Package httpapi wires the HTTP surface: the JSON API under /api and
// SPA static serving for everything else. Handlers stay thin; data access
// goes through the store package.
package httpapi

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Pinger reports whether the database is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewRouter builds the full HTTP handler. static holds the built SPA
// (index.html at its root); nil disables static serving (API-only).
func NewRouter(db Pinger, static fs.FS) http.Handler {
	r := chi.NewRouter()

	r.Get("/api/health", handleHealth(db))

	spa := spaHandler(static)
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if isReservedPath(req.URL.Path) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "no such endpoint")
			return
		}
		spa(w, req)
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
		if err := db.Ping(r.Context()); err != nil {
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
