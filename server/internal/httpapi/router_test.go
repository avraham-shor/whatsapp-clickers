package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(ctx context.Context) error { return s.err }

// stubAuth implements AuthService with canned results; loggedOut records
// every token passed to Logout so tests can assert server-side deletion.
type stubAuth struct {
	loginToken string
	loginOrg   auth.Organizer
	loginErr   error
	authOrg    auth.Organizer
	authErr    error
	logoutErr  error
	loggedOut  []string
}

func (s *stubAuth) Login(ctx context.Context, username, password string) (string, auth.Organizer, error) {
	return s.loginToken, s.loginOrg, s.loginErr
}

func (s *stubAuth) Authenticate(ctx context.Context, token string) (auth.Organizer, error) {
	return s.authOrg, s.authErr
}

func (s *stubAuth) Logout(ctx context.Context, token string) error {
	s.loggedOut = append(s.loggedOut, token)
	return s.logoutErr
}

// noAuth is the AuthService for tests that never touch auth routes.
func noAuth() *stubAuth { return &stubAuth{authErr: auth.ErrNoSession} }

func testStatic() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>app shell</html>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
	}
}

func decodeErrorCode(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("response %q is not the JSON error envelope: %v", body, err)
	}
	return envelope.Error.Code
}

func TestHealthOK(t *testing.T) {
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
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
	router := NewRouter(stubPinger{err: errors.New("connection refused")}, noAuth(), testStatic())
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
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
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
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
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
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
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
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
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
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
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
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
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
	router := NewRouter(stubPinger{}, noAuth(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET / with nil static = %d, want 404", rec.Code)
	}
}

func TestLoginSuccessSetsCookieAndReturnsOrganizer(t *testing.T) {
	svc := &stubAuth{
		loginToken: "raw-token-value",
		loginOrg:   auth.Organizer{ID: "org-1", Username: "avraham"},
	}
	router := NewRouter(stubPinger{}, svc, testStatic())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"avraham","password":"pw"}`))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/auth/login = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("login response is not valid JSON: %v", err)
	}
	if body.ID != "org-1" || body.Username != "avraham" {
		t.Errorf("login body = %+v, want org-1/avraham", body)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login set %d cookies, want exactly 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != "wc_session" || c.Value != "raw-token-value" {
		t.Errorf("cookie = %s=%s, want wc_session=raw-token-value", c.Name, c.Value)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie flags HttpOnly=%v Secure=%v SameSite=%v, want true/true/Lax", c.HttpOnly, c.Secure, c.SameSite)
	}
	if c.Path != "/" || c.MaxAge != 2592000 {
		t.Errorf("cookie Path=%q MaxAge=%d, want / and 2592000", c.Path, c.MaxAge)
	}
}

func TestLoginBadCredentialsReturns401Envelope(t *testing.T) {
	svc := &stubAuth{loginErr: auth.ErrInvalidCredentials}
	router := NewRouter(stubPinger{}, svc, testStatic())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"username":"avraham","password":"wrong"}`))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad-credentials login = %d, want 401", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "INVALID_CREDENTIALS" {
		t.Errorf("error code = %q, want INVALID_CREDENTIALS", code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("failed login must not set a session cookie")
	}
}

func TestLoginMalformedBodyReturns400(t *testing.T) {
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
	for name, body := range map[string]string{
		"not json":       "not-json",
		"empty":          "",
		"missing fields": `{"username":"avraham"}`,
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: POST /api/auth/login = %d, want 400", name, rec.Code)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "INVALID_REQUEST" {
			t.Errorf("%s: error code = %q, want INVALID_REQUEST", name, code)
		}
	}
}

func TestLoginBodyTooLargeReturns413(t *testing.T) {
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
	rec := httptest.NewRecorder()
	// Valid JSON shape, but padded past the 4 KiB body cap.
	body := `{"username":"avraham","password":"` + strings.Repeat("x", 8192) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized login body = %d, want 413", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "REQUEST_TOO_LARGE" {
		t.Errorf("error code = %q, want REQUEST_TOO_LARGE", code)
	}
}

func TestLoginFailureCausesMapToDistinctCodes(t *testing.T) {
	// Token minting is an internal fault (500); everything else unexpected
	// stays an infrastructure 503. One generic bucket misleads triage.
	for name, tc := range map[string]struct {
		err      error
		wantCode int
		wantBody string
	}{
		"token generation": {
			err:      fmt.Errorf("%w: entropy pool sad", auth.ErrTokenGeneration),
			wantCode: http.StatusInternalServerError,
			wantBody: "INTERNAL",
		},
		"db failure": {
			err:      errors.New("look up organizer: connection refused"),
			wantCode: http.StatusServiceUnavailable,
			wantBody: "DB_UNAVAILABLE",
		},
	} {
		svc := &stubAuth{loginErr: tc.err}
		router := NewRouter(stubPinger{}, svc, testStatic())
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
			strings.NewReader(`{"username":"avraham","password":"pw"}`))
		router.ServeHTTP(rec, req)

		if rec.Code != tc.wantCode {
			t.Errorf("%s: login = %d, want %d", name, rec.Code, tc.wantCode)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != tc.wantBody {
			t.Errorf("%s: error code = %q, want %q", name, code, tc.wantBody)
		}
	}
}

func TestMeWithoutCookieReturns401Envelope(t *testing.T) {
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/auth/me without cookie = %d, want 401", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "UNAUTHORIZED" {
		t.Errorf("error code = %q, want UNAUTHORIZED", code)
	}
}

func TestMeWithValidCookieReturnsOrganizer(t *testing.T) {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	router := NewRouter(stubPinger{}, svc, testStatic())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "wc_session", Value: "raw-token-value"})
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/auth/me with cookie = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("me response is not valid JSON: %v", err)
	}
	if body.ID != "org-1" || body.Username != "avraham" {
		t.Errorf("me body = %+v, want org-1/avraham", body)
	}
}

func TestExpiredSessionReturns401(t *testing.T) {
	// The SQL lookup filters expired rows, so auth reports ErrNoSession.
	svc := &stubAuth{authErr: auth.ErrNoSession}
	router := NewRouter(stubPinger{}, svc, testStatic())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "wc_session", Value: "stale-token"})
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/auth/me with expired session = %d, want 401", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "UNAUTHORIZED" {
		t.Errorf("error code = %q, want UNAUTHORIZED", code)
	}
}

func TestLogoutDeletesSessionAndClearsCookie(t *testing.T) {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	router := NewRouter(stubPinger{}, svc, testStatic())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "wc_session", Value: "raw-token-value"})
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /api/auth/logout = %d, want 204 (body %s)", rec.Code, rec.Body)
	}
	if len(svc.loggedOut) != 1 || svc.loggedOut[0] != "raw-token-value" {
		t.Errorf("Logout called with %v, want exactly [raw-token-value]", svc.loggedOut)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("logout set %d cookies, want exactly 1 (the clearing one)", len(cookies))
	}
	c := cookies[0]
	if c.Name != "wc_session" || c.Value != "" || c.MaxAge >= 0 {
		t.Errorf("clearing cookie = %s=%q MaxAge=%d, want empty wc_session with Max-Age=0", c.Name, c.Value, c.MaxAge)
	}
	if !strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Errorf("Set-Cookie %q does not carry Max-Age=0", rec.Header().Get("Set-Cookie"))
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Errorf("clearing cookie must keep the same attributes: HttpOnly=%v Secure=%v SameSite=%v Path=%q", c.HttpOnly, c.Secure, c.SameSite, c.Path)
	}
}

func TestLogoutFailureStillClearsCookie(t *testing.T) {
	// If the server-side delete fails the browser must not keep a cookie it
	// believes was cleared — clear it anyway and report the 503.
	svc := &stubAuth{
		authOrg:   auth.Organizer{ID: "org-1", Username: "avraham"},
		logoutErr: errors.New("connection refused"),
	}
	router := NewRouter(stubPinger{}, svc, testStatic())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "wc_session", Value: "raw-token-value"})
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("failing logout = %d, want 503", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "DB_UNAVAILABLE" {
		t.Errorf("error code = %q, want DB_UNAVAILABLE", code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("failing logout set %d cookies, want exactly 1 (the clearing one)", len(cookies))
	}
	if c := cookies[0]; c.Name != "wc_session" || c.Value != "" || c.MaxAge >= 0 {
		t.Errorf("clearing cookie = %s=%q MaxAge=%d, want empty wc_session with Max-Age=0", c.Name, c.Value, c.MaxAge)
	}
}

func TestLogoutWithoutSessionReturns401(t *testing.T) {
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/auth/logout without session = %d, want 401", rec.Code)
	}
}

func TestHealthStaysPublic(t *testing.T) {
	// Health must never sit behind RequireOrganizer (Railway monitoring).
	router := NewRouter(stubPinger{}, noAuth(), testStatic())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health without session = %d, want 200", rec.Code)
	}
}
