package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
	"github.com/avraham-shor/whatsapp-clickers/internal/game"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
)

// stubLobbyEngine implements LobbyEngine with a canned result; requestedFor
// records every (gameID, organizerID) pair asked for.
type stubLobbyEngine struct {
	snapshot     game.Snapshot
	err          error
	requestedFor [][2]string
}

func (s *stubLobbyEngine) OpenLobby(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.requestedFor = append(s.requestedFor, [2]string{gameID, organizerID})
	return s.snapshot, s.err
}

// stubBroadcaster implements SnapshotBroadcaster and records every call.
type stubBroadcaster struct {
	calls []broadcastCall
}

type broadcastCall struct {
	gameID   string
	snapshot game.Snapshot
}

func (s *stubBroadcaster) Broadcast(gameID string, snapshot game.Snapshot) {
	s.calls = append(s.calls, broadcastCall{gameID, snapshot})
}

// controlRouter builds a router with an authenticated org-1 session and the
// given LobbyEngine/SnapshotBroadcaster stubs.
func controlRouter(engine LobbyEngine, hub SnapshotBroadcaster) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil)
}

func TestOpenLobbySuccessReturnsSnapshotAndBroadcasts(t *testing.T) {
	engine := &stubLobbyEngine{snapshot: game.Snapshot{
		GameID: testGameID, State: "lobby", JoinCode: "AB2CD3", PlatformNumber: "+972 50-000-0000",
		Participants: []game.ParticipantSummary{},
	}}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/open-lobby", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /open-lobby = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		GameID   string `json:"gameId"`
		State    string `json:"state"`
		JoinCode string `json:"joinCode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.GameID != testGameID || body.State != "lobby" || body.JoinCode != "AB2CD3" {
		t.Errorf("body = %+v, want the engine's snapshot", body)
	}
	if len(engine.requestedFor) != 1 || engine.requestedFor[0] != [2]string{testGameID, "org-1"} {
		t.Errorf("engine.OpenLobby called with %v, want [[%s org-1]]", engine.requestedFor, testGameID)
	}
	if len(hub.calls) != 1 || hub.calls[0].gameID != testGameID {
		t.Errorf("Broadcast called %+v, want exactly one call for %s", hub.calls, testGameID)
	}
}

func TestOpenLobbyNonDraftReturns409WithoutBroadcasting(t *testing.T) {
	engine := &stubLobbyEngine{err: game.ErrNotDraft}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/open-lobby", ""))

	if rec.Code != http.StatusConflict {
		t.Fatalf("POST /open-lobby on non-draft game = %d, want 409", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_EDITABLE" {
		t.Errorf("error code = %q, want GAME_NOT_EDITABLE", code)
	}
	if len(hub.calls) != 0 {
		t.Error("Broadcast called on a rejected transition")
	}
}

func TestOpenLobbyForeignOrMissingGameReturns404(t *testing.T) {
	engine := &stubLobbyEngine{err: store.ErrNotFound}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/open-lobby", ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /open-lobby on foreign/missing game = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
	if len(hub.calls) != 0 {
		t.Error("Broadcast called on a failed transition")
	}
}

func TestOpenLobbyWithoutSessionReturns401(t *testing.T) {
	engine := &stubLobbyEngine{}
	hub := &stubBroadcaster{}
	router := NewRouter(stubPinger{}, noAuth(), noGames(), testStatic(), nil, engine, hub, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/games/"+testGameID+"/open-lobby", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /open-lobby without session = %d, want 401", rec.Code)
	}
	if len(engine.requestedFor) != 0 {
		t.Error("unauthenticated request reached the engine")
	}
}
