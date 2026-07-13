package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(ctx context.Context) error { return s.err }

func testStatic() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>app shell</html>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
	}
}

func TestHealthOK(t *testing.T) {
	router := NewRouter(stubPinger{}, testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body["status"] != "ok" || body["db"] != "ok" {
		t.Errorf(`body = %v, want {"status":"ok","db":"ok"}`, body)
	}
}

func TestHealthDBDown(t *testing.T) {
	router := NewRouter(stubPinger{err: errors.New("connection refused")}, testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /api/health with dead DB = %d, want 503", rec.Code)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error response is not valid JSON: %v", err)
	}
	if body.Error.Code != "DB_UNAVAILABLE" {
		t.Errorf("error code = %q, want DB_UNAVAILABLE", body.Error.Code)
	}
}

func TestSPARootServesIndex(t *testing.T) {
	router := NewRouter(stubPinger{}, testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if got, _ := io.ReadAll(rec.Body); !strings.Contains(string(got), "app shell") {
		t.Errorf("GET / body = %q, want index.html content", got)
	}
}

func TestSPAStaticAssetServed(t *testing.T) {
	router := NewRouter(stubPinger{}, testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, "console.log") {
		t.Errorf("asset body = %q, want file content", got)
	}
}

func TestSPAClientRouteFallsBackToIndex(t *testing.T) {
	router := NewRouter(stubPinger{}, testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/display/abc123", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /display/abc123 = %d, want 200 (SPA fallback)", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, "app shell") {
		t.Errorf("fallback body = %q, want index.html content", got)
	}
}

func TestMissingAssetReturns404(t *testing.T) {
	router := NewRouter(stubPinger{}, testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/index-OLDHASH.js", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET missing asset = %d, want 404 (no index.html fallback)", rec.Code)
	}
	if got := rec.Body.String(); strings.Contains(got, "app shell") {
		t.Errorf("missing asset served index.html content: %q", got)
	}
}

func TestMethodNotAllowedReturnsJSONError(t *testing.T) {
	router := NewRouter(stubPinger{}, testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/health", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/health = %d, want 405", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("405 body is not JSON envelope: %v", err)
	}
	if body.Error.Code != "METHOD_NOT_ALLOWED" {
		t.Errorf("error code = %q, want METHOD_NOT_ALLOWED", body.Error.Code)
	}
}

func TestUnknownAPIRouteReturnsJSONError(t *testing.T) {
	router := NewRouter(stubPinger{}, testStatic())
	for _, path := range []string{"/api/nope", "/ws", "/webhooks/whatsapp"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (no SPA fallback)", path, rec.Code)
			continue
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Errorf("GET %s: 404 body is not JSON envelope: %v", path, err)
			continue
		}
		if body.Error.Code != "NOT_FOUND" {
			t.Errorf("GET %s error code = %q, want NOT_FOUND", path, body.Error.Code)
		}
	}
}

func TestNilStaticReturns404ForSPARoutes(t *testing.T) {
	router := NewRouter(stubPinger{}, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET / with nil static = %d, want 404", rec.Code)
	}
}
