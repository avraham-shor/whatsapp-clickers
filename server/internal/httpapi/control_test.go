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

// stubControlEngine implements ControlEngine with a canned result per
// method; each *RequestedFor slice records every (gameID, organizerID)
// pair the corresponding method was asked for.
type stubControlEngine struct {
	openLobbySnapshot     game.Snapshot
	openLobbyErr          error
	openLobbyRequestedFor [][2]string

	startGameSnapshot     game.Snapshot
	startGameErr          error
	startGameRequestedFor [][2]string

	closeQuestionSnapshot     game.Snapshot
	closeQuestionErr          error
	closeQuestionRequestedFor [][2]string

	revealSnapshot     game.Snapshot
	revealErr          error
	revealRequestedFor [][2]string

	nextQuestionSnapshot     game.Snapshot
	nextQuestionErr          error
	nextQuestionRequestedFor [][2]string

	stopGameSnapshot     game.Snapshot
	stopGameErr          error
	stopGameRequestedFor [][2]string
}

func (s *stubControlEngine) OpenLobby(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.openLobbyRequestedFor = append(s.openLobbyRequestedFor, [2]string{gameID, organizerID})
	return s.openLobbySnapshot, s.openLobbyErr
}

func (s *stubControlEngine) StartGame(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.startGameRequestedFor = append(s.startGameRequestedFor, [2]string{gameID, organizerID})
	return s.startGameSnapshot, s.startGameErr
}

func (s *stubControlEngine) CloseQuestion(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.closeQuestionRequestedFor = append(s.closeQuestionRequestedFor, [2]string{gameID, organizerID})
	return s.closeQuestionSnapshot, s.closeQuestionErr
}

func (s *stubControlEngine) Reveal(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.revealRequestedFor = append(s.revealRequestedFor, [2]string{gameID, organizerID})
	return s.revealSnapshot, s.revealErr
}

func (s *stubControlEngine) NextQuestion(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.nextQuestionRequestedFor = append(s.nextQuestionRequestedFor, [2]string{gameID, organizerID})
	return s.nextQuestionSnapshot, s.nextQuestionErr
}

func (s *stubControlEngine) StopGame(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.stopGameRequestedFor = append(s.stopGameRequestedFor, [2]string{gameID, organizerID})
	return s.stopGameSnapshot, s.stopGameErr
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
// given ControlEngine/SnapshotBroadcaster stubs.
func controlRouter(engine ControlEngine, hub SnapshotBroadcaster) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil)
}

func TestOpenLobbySuccessReturnsSnapshotAndBroadcasts(t *testing.T) {
	engine := &stubControlEngine{openLobbySnapshot: game.Snapshot{
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
	if len(engine.openLobbyRequestedFor) != 1 || engine.openLobbyRequestedFor[0] != [2]string{testGameID, "org-1"} {
		t.Errorf("engine.OpenLobby called with %v, want [[%s org-1]]", engine.openLobbyRequestedFor, testGameID)
	}
	if len(hub.calls) != 1 || hub.calls[0].gameID != testGameID {
		t.Errorf("Broadcast called %+v, want exactly one call for %s", hub.calls, testGameID)
	}
}

func TestOpenLobbyNonDraftReturns409WithoutBroadcasting(t *testing.T) {
	engine := &stubControlEngine{openLobbyErr: game.ErrNotDraft}
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
	engine := &stubControlEngine{openLobbyErr: store.ErrNotFound}
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
	engine := &stubControlEngine{}
	hub := &stubBroadcaster{}
	router := NewRouter(stubPinger{}, noAuth(), noGames(), testStatic(), nil, engine, hub, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/games/"+testGameID+"/open-lobby", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /open-lobby without session = %d, want 401", rec.Code)
	}
	if len(engine.openLobbyRequestedFor) != 0 {
		t.Error("unauthenticated request reached the engine")
	}
}

// controlActionCase describes one of the five live-control actions this
// story adds (everything past open-lobby), so the success/error/404/401
// suite below (mirroring the TestOpenLobby* suite above) runs once per
// action instead of being duplicated by hand five times.
type controlActionCase struct {
	name         string
	path         string
	domainErr    error
	domainCode   string
	setSnapshot  func(*stubControlEngine, game.Snapshot)
	setErr       func(*stubControlEngine, error)
	requestedFor func(*stubControlEngine) [][2]string
}

var controlActionCases = []controlActionCase{
	{
		name:         "StartGame",
		path:         "/start",
		domainErr:    game.ErrNotLobby,
		domainCode:   "GAME_NOT_LOBBY",
		setSnapshot:  func(e *stubControlEngine, s game.Snapshot) { e.startGameSnapshot = s },
		setErr:       func(e *stubControlEngine, err error) { e.startGameErr = err },
		requestedFor: func(e *stubControlEngine) [][2]string { return e.startGameRequestedFor },
	},
	{
		name:         "CloseQuestion",
		path:         "/close-question",
		domainErr:    game.ErrNotQuestionOpen,
		domainCode:   "GAME_NOT_QUESTION_OPEN",
		setSnapshot:  func(e *stubControlEngine, s game.Snapshot) { e.closeQuestionSnapshot = s },
		setErr:       func(e *stubControlEngine, err error) { e.closeQuestionErr = err },
		requestedFor: func(e *stubControlEngine) [][2]string { return e.closeQuestionRequestedFor },
	},
	{
		name:         "Reveal",
		path:         "/reveal",
		domainErr:    game.ErrNotQuestionClosed,
		domainCode:   "GAME_NOT_QUESTION_CLOSED",
		setSnapshot:  func(e *stubControlEngine, s game.Snapshot) { e.revealSnapshot = s },
		setErr:       func(e *stubControlEngine, err error) { e.revealErr = err },
		requestedFor: func(e *stubControlEngine) [][2]string { return e.revealRequestedFor },
	},
	{
		name:         "NextQuestion",
		path:         "/next-question",
		domainErr:    game.ErrNotRevealed,
		domainCode:   "GAME_NOT_REVEALED",
		setSnapshot:  func(e *stubControlEngine, s game.Snapshot) { e.nextQuestionSnapshot = s },
		setErr:       func(e *stubControlEngine, err error) { e.nextQuestionErr = err },
		requestedFor: func(e *stubControlEngine) [][2]string { return e.nextQuestionRequestedFor },
	},
	{
		name:         "StopGame",
		path:         "/stop",
		domainErr:    game.ErrNotStoppable,
		domainCode:   "GAME_NOT_STOPPABLE",
		setSnapshot:  func(e *stubControlEngine, s game.Snapshot) { e.stopGameSnapshot = s },
		setErr:       func(e *stubControlEngine, err error) { e.stopGameErr = err },
		requestedFor: func(e *stubControlEngine) [][2]string { return e.stopGameRequestedFor },
	},
}

func TestControlActionSuccessReturnsSnapshotAndBroadcasts(t *testing.T) {
	for _, tc := range controlActionCases {
		t.Run(tc.name, func(t *testing.T) {
			engine := &stubControlEngine{}
			tc.setSnapshot(engine, game.Snapshot{GameID: testGameID, State: "question_open"})
			hub := &stubBroadcaster{}
			rec := httptest.NewRecorder()
			controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+tc.path, ""))

			if rec.Code != http.StatusOK {
				t.Fatalf("POST %s = %d, want 200 (body %s)", tc.path, rec.Code, rec.Body)
			}
			var body struct {
				GameID string `json:"gameId"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}
			if body.GameID != testGameID {
				t.Errorf("body.GameID = %q, want %q", body.GameID, testGameID)
			}
			if got := tc.requestedFor(engine); len(got) != 1 || got[0] != [2]string{testGameID, "org-1"} {
				t.Errorf("engine.%s called with %v, want [[%s org-1]]", tc.name, got, testGameID)
			}
			if len(hub.calls) != 1 || hub.calls[0].gameID != testGameID {
				t.Errorf("Broadcast called %+v, want exactly one call for %s", hub.calls, testGameID)
			}
		})
	}
}

func TestControlActionDomainErrorReturns409WithoutBroadcasting(t *testing.T) {
	for _, tc := range controlActionCases {
		t.Run(tc.name, func(t *testing.T) {
			engine := &stubControlEngine{}
			tc.setErr(engine, tc.domainErr)
			hub := &stubBroadcaster{}
			rec := httptest.NewRecorder()
			controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+tc.path, ""))

			if rec.Code != http.StatusConflict {
				t.Fatalf("POST %s on rejected transition = %d, want 409", tc.path, rec.Code)
			}
			if code := decodeErrorCode(t, rec.Body.Bytes()); code != tc.domainCode {
				t.Errorf("error code = %q, want %q", code, tc.domainCode)
			}
			if len(hub.calls) != 0 {
				t.Error("Broadcast called on a rejected transition")
			}
		})
	}
}

func TestControlActionForeignOrMissingGameReturns404(t *testing.T) {
	for _, tc := range controlActionCases {
		t.Run(tc.name, func(t *testing.T) {
			engine := &stubControlEngine{}
			tc.setErr(engine, store.ErrNotFound)
			hub := &stubBroadcaster{}
			rec := httptest.NewRecorder()
			controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+tc.path, ""))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("POST %s on foreign/missing game = %d, want 404", tc.path, rec.Code)
			}
			if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
				t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
			}
			if len(hub.calls) != 0 {
				t.Error("Broadcast called on a failed transition")
			}
		})
	}
}

func TestControlActionWithoutSessionReturns401(t *testing.T) {
	for _, tc := range controlActionCases {
		t.Run(tc.name, func(t *testing.T) {
			engine := &stubControlEngine{}
			hub := &stubBroadcaster{}
			router := NewRouter(stubPinger{}, noAuth(), noGames(), testStatic(), nil, engine, hub, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/games/"+testGameID+tc.path, nil))

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("POST %s without session = %d, want 401", tc.path, rec.Code)
			}
			if got := tc.requestedFor(engine); len(got) != 0 {
				t.Errorf("unauthenticated request reached the engine: %v", got)
			}
		})
	}
}

func TestStartGameNoQuestionsReturns409(t *testing.T) {
	// StartGame's second rejection reason (distinct from GAME_NOT_LOBBY,
	// covered by the table above) — a lobby game with an empty roster.
	engine := &stubControlEngine{startGameErr: game.ErrNoQuestions}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/start", ""))

	if rec.Code != http.StatusConflict {
		t.Fatalf("POST /start with no questions = %d, want 409", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NO_QUESTIONS" {
		t.Errorf("error code = %q, want GAME_NO_QUESTIONS", code)
	}
	if len(hub.calls) != 0 {
		t.Error("Broadcast called on a rejected transition")
	}
}
