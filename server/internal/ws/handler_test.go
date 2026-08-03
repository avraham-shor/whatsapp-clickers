package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
	"github.com/avraham-shor/whatsapp-clickers/internal/game"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
)

const testGameID = "11111111-2222-3333-4444-555555555555"

// stubAuth implements Authenticator with a canned result.
type stubAuth struct {
	org auth.Organizer
	err error
}

func (s stubAuth) Authenticate(ctx context.Context, token string) (auth.Organizer, error) {
	return s.org, s.err
}

// stubEngine implements SnapshotReader with a canned result; requestedFor
// records every (gameID, organizerID) pair asked for. If snapshots is set,
// each call consumes the next entry (repeating the last once exhausted) —
// used to simulate the handler's pre-register/post-register refetch seeing
// a different snapshot the second time. onSnapshot, if set, runs before the
// canned result is returned, keyed by the call's index (0-based) — used to
// inject a hub.Broadcast mid-call, simulating a broadcast racing the
// snapshot build.
type stubEngine struct {
	snapshot     game.Snapshot
	snapshots    []game.Snapshot
	err          error
	requestedFor [][2]string
	onSnapshot   func(call int)
}

func (s *stubEngine) Snapshot(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	call := len(s.requestedFor)
	s.requestedFor = append(s.requestedFor, [2]string{gameID, organizerID})
	if s.onSnapshot != nil {
		s.onSnapshot(call)
	}
	if s.err != nil {
		return game.Snapshot{}, s.err
	}
	if len(s.snapshots) > 0 {
		if call < len(s.snapshots) {
			return s.snapshots[call], nil
		}
		return s.snapshots[len(s.snapshots)-1], nil
	}
	return s.snapshot, nil
}

func authedOrg() stubAuth {
	return stubAuth{org: auth.Organizer{ID: "org-1", Username: "avraham"}}
}

// dialWS connects to the test server's /ws endpoint with the given query
// string and (optionally) a session cookie; the caller closes the
// connection. httptest.NewRecorder cannot be used here — websocket.Accept
// requires the ResponseWriter to implement http.Hijacker, which a recorder
// does not.
func dialWS(t *testing.T, srv *httptest.Server, query string, withCookie bool) (*websocket.Conn, *dialResult) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws" + query
	opts := &websocket.DialOptions{}
	if withCookie {
		opts.HTTPHeader = map[string][]string{"Cookie": {"wc_session=raw-token-value"}}
	}
	conn, resp, err := websocket.Dial(ctx, u, opts)
	if err != nil {
		// The handshake response is still populated on failure (per
		// coder/websocket's Dial doc) — capture it so rejection tests can
		// pin the actual status code rather than only "dial failed", which
		// would also pass for a handler rejecting for the wrong reason
		// (e.g. a 500 instead of the intended 401/404/400).
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return nil, &dialResult{err: err, statusCode: statusCode}
	}
	return conn, &dialResult{statusCode: resp.StatusCode}
}

// dialResult carries just enough of the dial response for assertions
// without pulling in the full http.Response plumbing at call sites.
type dialResult struct {
	statusCode int
	err        error
}

func TestValidConnectionReceivesInitialSnapshot(t *testing.T) {
	engine := &stubEngine{snapshot: game.Snapshot{
		GameID: testGameID, State: "lobby", JoinCode: "AB2CD3", PlatformNumber: "+972 50-000-0000",
		Participants: []game.ParticipantSummary{},
	}}
	hub := NewHub(nil)
	srv := httptest.NewServer(NewHandler(authedOrg(), engine, hub, nil))
	defer srv.Close()

	conn, resp := dialWS(t, srv, "?gameId="+testGameID+"&role=host", true)
	if resp.err != nil {
		t.Fatalf("dial failed: %v", resp.err)
	}
	defer conn.CloseNow()

	readCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read first frame: %v", err)
	}
	env := decodeEnvelope(t, data)
	if env.Type != "snapshot" || env.State.GameID != testGameID || env.State.State != "lobby" {
		t.Errorf("first frame = %+v, want a snapshot envelope for %s", env, testGameID)
	}
	if env.Seq != 0 {
		// No Broadcast has fired for this game yet, so the hub's current
		// seq (what the initial frame is stamped with) is still its zero
		// value.
		t.Errorf("first frame Seq = %d, want 0 (no broadcast has fired yet)", env.Seq)
	}
	if len(engine.requestedFor) != 1 || engine.requestedFor[0] != [2]string{testGameID, "org-1"} {
		t.Errorf("engine.Snapshot called with %v, want [[%s org-1]]", engine.requestedFor, testGameID)
	}
}

func TestInitialFrameRefreshesWhenABroadcastRacesRegistration(t *testing.T) {
	// A Broadcast landing between the handler's engine.Snapshot() read and
	// its hub.register() call must not ship the now-stale pre-race
	// snapshot stamped with a seq that looks current — the handler is
	// expected to detect the hub's seq moved and refetch.
	hub := NewHub(nil)
	stale := game.Snapshot{GameID: testGameID, State: "draft"}
	fresh := game.Snapshot{GameID: testGameID, State: "lobby"}
	engine := &stubEngine{snapshots: []game.Snapshot{stale, fresh}}
	engine.onSnapshot = func(call int) {
		if call == 0 {
			hub.Broadcast(testGameID, fresh)
		}
	}
	srv := httptest.NewServer(NewHandler(authedOrg(), engine, hub, nil))
	defer srv.Close()

	conn, resp := dialWS(t, srv, "?gameId="+testGameID+"&role=host", true)
	if resp.err != nil {
		t.Fatalf("dial failed: %v", resp.err)
	}
	defer conn.CloseNow()

	readCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read first frame: %v", err)
	}
	env := decodeEnvelope(t, data)
	if env.State.State != "lobby" {
		t.Errorf("first frame state = %q, want the fresh post-race snapshot (lobby), not the stale pre-race one (draft)", env.State.State)
	}
	if env.Seq != 1 {
		t.Errorf("first frame Seq = %d, want 1 (paired with the fresh snapshot, matching the broadcast that raced it)", env.Seq)
	}
	if len(engine.requestedFor) != 2 {
		t.Errorf("engine.Snapshot called %d times, want 2 (initial read + post-register refresh)", len(engine.requestedFor))
	}
}

func TestMissingCookieRejectsHandshake(t *testing.T) {
	hub := NewHub(nil)
	srv := httptest.NewServer(NewHandler(authedOrg(), &stubEngine{}, hub, nil))
	defer srv.Close()

	conn, resp := dialWS(t, srv, "?gameId="+testGameID+"&role=host", false)
	if resp.err == nil {
		conn.CloseNow()
		t.Fatal("dial succeeded without a session cookie, want rejection")
	}
	if resp.statusCode != http.StatusUnauthorized {
		t.Errorf("rejection status = %d, want 401 (not a handler failing for the wrong reason)", resp.statusCode)
	}
}

func TestInvalidSessionRejectsHandshake(t *testing.T) {
	hub := NewHub(nil)
	svc := stubAuth{err: auth.ErrNoSession}
	srv := httptest.NewServer(NewHandler(svc, &stubEngine{}, hub, nil))
	defer srv.Close()

	conn, resp := dialWS(t, srv, "?gameId="+testGameID+"&role=host", true)
	if resp.err == nil {
		conn.CloseNow()
		t.Fatal("dial succeeded with an invalid session, want rejection")
	}
	if resp.statusCode != http.StatusUnauthorized {
		t.Errorf("rejection status = %d, want 401 (not a handler failing for the wrong reason)", resp.statusCode)
	}
}

func TestForeignOrMissingGameRejectsHandshake(t *testing.T) {
	hub := NewHub(nil)
	engine := &stubEngine{err: store.ErrNotFound}
	srv := httptest.NewServer(NewHandler(authedOrg(), engine, hub, nil))
	defer srv.Close()

	conn, resp := dialWS(t, srv, "?gameId="+testGameID+"&role=host", true)
	if resp.err == nil {
		conn.CloseNow()
		t.Fatal("dial succeeded for a foreign/missing game, want rejection")
	}
	if resp.statusCode != http.StatusNotFound {
		t.Errorf("rejection status = %d, want 404 (not a handler failing for the wrong reason)", resp.statusCode)
	}
}

func TestMalformedGameIDRejectsHandshakeCleanly(t *testing.T) {
	hub := NewHub(nil)
	engine := &stubEngine{}
	srv := httptest.NewServer(NewHandler(authedOrg(), engine, hub, nil))
	defer srv.Close()

	conn, resp := dialWS(t, srv, "?gameId=not-a-uuid&role=host", true)
	if resp.err == nil {
		conn.CloseNow()
		t.Fatal("dial succeeded with a malformed gameId, want rejection")
	}
	if resp.statusCode != http.StatusNotFound {
		t.Errorf("rejection status = %d, want 404 (clean rejection, never a 503 from a raw DB type error)", resp.statusCode)
	}
	if len(engine.requestedFor) != 0 {
		t.Error("malformed gameId reached the engine instead of short-circuiting")
	}
}

func TestInvalidRoleRejectsHandshake(t *testing.T) {
	hub := NewHub(nil)
	engine := &stubEngine{snapshot: game.Snapshot{GameID: testGameID}}
	srv := httptest.NewServer(NewHandler(authedOrg(), engine, hub, nil))
	defer srv.Close()

	conn, resp := dialWS(t, srv, "?gameId="+testGameID+"&role=participant", true)
	if resp.err == nil {
		conn.CloseNow()
		t.Fatal("dial succeeded with role=participant, want rejection")
	}
	if resp.statusCode != http.StatusBadRequest {
		t.Errorf("rejection status = %d, want 400 (not a handler failing for the wrong reason)", resp.statusCode)
	}
	if len(engine.requestedFor) != 0 {
		t.Error("invalid role reached the engine instead of short-circuiting")
	}
}

func TestTwoConnectionsBothReceiveALaterBroadcast(t *testing.T) {
	engine := &stubEngine{snapshot: game.Snapshot{GameID: testGameID, State: "lobby"}}
	hub := NewHub(nil)
	srv := httptest.NewServer(NewHandler(authedOrg(), engine, hub, nil))
	defer srv.Close()

	conn1, resp1 := dialWS(t, srv, "?gameId="+testGameID+"&role=host", true)
	if resp1.err != nil {
		t.Fatalf("dial 1 failed: %v", resp1.err)
	}
	defer conn1.CloseNow()
	conn2, resp2 := dialWS(t, srv, "?gameId="+testGameID+"&role=display", true)
	if resp2.err != nil {
		t.Fatalf("dial 2 failed: %v", resp2.err)
	}
	defer conn2.CloseNow()

	readCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Drain each connection's initial snapshot before broadcasting.
	if _, _, err := conn1.Read(readCtx); err != nil {
		t.Fatalf("read conn1 initial frame: %v", err)
	}
	if _, _, err := conn2.Read(readCtx); err != nil {
		t.Fatalf("read conn2 initial frame: %v", err)
	}

	hub.Broadcast(testGameID, game.Snapshot{GameID: testGameID, State: "question_open"})

	for i, conn := range []*websocket.Conn{conn1, conn2} {
		_, data, err := conn.Read(readCtx)
		if err != nil {
			t.Fatalf("conn%d read broadcast frame: %v", i+1, err)
		}
		env := decodeEnvelope(t, data)
		if env.State.State != "question_open" {
			t.Errorf("conn%d broadcast frame state = %q, want question_open", i+1, env.State.State)
		}
	}
}
