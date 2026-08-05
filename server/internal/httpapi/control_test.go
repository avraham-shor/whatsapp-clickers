package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

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

	// playerRecipientsMu guards the fields below. dispatchQuestionOpened
	// (control.go) now runs in a goroutine spawned after the HTTP response
	// is written, so PlayerRecipients can run concurrently with a test
	// goroutine reading these fields. playerRecipientsDone, if non-nil,
	// receives a signal after every call — tests exercising the dispatch
	// goroutine wait on it instead of racing or sleeping.
	playerRecipientsMu           sync.Mutex
	playerRecipients             []string
	playerRecipientsErr          error
	playerRecipientsRequestedFor []string
	playerRecipientsDone         chan struct{}
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

func (s *stubControlEngine) PlayerRecipients(ctx context.Context, gameID string) ([]string, error) {
	s.playerRecipientsMu.Lock()
	s.playerRecipientsRequestedFor = append(s.playerRecipientsRequestedFor, gameID)
	recipients, err := s.playerRecipients, s.playerRecipientsErr
	s.playerRecipientsMu.Unlock()
	if s.playerRecipientsDone != nil {
		s.playerRecipientsDone <- struct{}{}
	}
	return recipients, err
}

// PlayerRecipientsRequestedFor returns a thread-safe snapshot of every
// gameID PlayerRecipients was called with.
func (s *stubControlEngine) PlayerRecipientsRequestedFor() []string {
	s.playerRecipientsMu.Lock()
	defer s.playerRecipientsMu.Unlock()
	return append([]string(nil), s.playerRecipientsRequestedFor...)
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

// stubQuestionDispatcher implements QuestionDispatcher and records every
// DispatchQuestionOpened call. Dispatch now runs in a goroutine the handler
// spawns after writing its HTTP response (see dispatchQuestionOpened's doc
// comment in control.go), so calls is guarded by mu; done, if non-nil,
// receives a signal after every recorded call so tests can wait
// deterministically instead of racing the goroutine or sleeping.
type stubQuestionDispatcher struct {
	mu    sync.Mutex
	calls []dispatchCall
	done  chan struct{}
}

type dispatchCall struct {
	gameID        string
	question      game.CurrentQuestion
	questionCount int
	recipients    []string
}

func (s *stubQuestionDispatcher) DispatchQuestionOpened(gameID string, question game.CurrentQuestion, questionCount int, recipients []string) {
	s.mu.Lock()
	s.calls = append(s.calls, dispatchCall{gameID: gameID, question: question, questionCount: questionCount, recipients: recipients})
	s.mu.Unlock()
	if s.done != nil {
		s.done <- struct{}{}
	}
}

// Calls returns a thread-safe snapshot of every recorded call.
func (s *stubQuestionDispatcher) Calls() []dispatchCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]dispatchCall(nil), s.calls...)
}

// waitForSignal blocks until ch fires or the test times out — used to
// observe the dispatch goroutine handleStartGame/handleNextQuestion spawn
// after writing their HTTP response, without sleeping or racing it.
func waitForSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the dispatch goroutine")
	}
}

// controlRouter builds a router with an authenticated org-1 session and the
// given ControlEngine/SnapshotBroadcaster stubs. No QuestionDispatcher —
// none of this file's existing tests exercise WhatsApp dispatch.
func controlRouter(engine ControlEngine, hub SnapshotBroadcaster) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil, nil)
}

// controlRouterWithDispatcher is controlRouter plus a real QuestionDispatcher
// — used only by the dispatch tests below.
func controlRouterWithDispatcher(engine ControlEngine, hub SnapshotBroadcaster, dispatcher QuestionDispatcher) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil, dispatcher)
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
	router := NewRouter(stubPinger{}, noAuth(), noGames(), testStatic(), nil, engine, hub, nil, nil)
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

// TestHandleRevealGradingIncompleteReturns409WithoutBroadcasting covers
// Reveal's second rejection (story 3.4, FR-16 epic AC-3) — a distinct
// GRADING_INCOMPLETE 409, not folded into controlActionCases' one-
// representative-error-per-endpoint table above (that table already
// covers Reveal's other rejection, ErrNotQuestionClosed).
func TestHandleRevealGradingIncompleteReturns409WithoutBroadcasting(t *testing.T) {
	engine := &stubControlEngine{revealErr: game.ErrGradingIncomplete}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/reveal", ""))

	if rec.Code != http.StatusConflict {
		t.Fatalf("POST /reveal on grading-incomplete = %d, want 409", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GRADING_INCOMPLETE" {
		t.Errorf("error code = %q, want GRADING_INCOMPLETE", code)
	}
	if len(hub.calls) != 0 {
		t.Error("Broadcast called on a rejected transition")
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
			router := NewRouter(stubPinger{}, noAuth(), noGames(), testStatic(), nil, engine, hub, nil, nil)
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

// --- Question dispatch (story 3.2) ---

func TestStartGameDispatchesQuestionToPlayers(t *testing.T) {
	question := game.CurrentQuestion{ID: "q1", Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20}
	wantRecipients := []string{"+972500000001", "+972500000002"}
	engine := &stubControlEngine{
		startGameSnapshot: game.Snapshot{GameID: testGameID, State: "question_open", QuestionCount: 3, CurrentQuestion: &question},
		playerRecipients:  wantRecipients,
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubQuestionDispatcher{done: make(chan struct{}, 1)}
	rec := httptest.NewRecorder()
	controlRouterWithDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/start", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /start = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	waitForSignal(t, dispatcher.done)

	calls := dispatcher.Calls()
	if len(calls) != 1 {
		t.Fatalf("DispatchQuestionOpened called %d times, want 1", len(calls))
	}
	call := calls[0]
	if call.gameID != testGameID || call.question.ID != "q1" || call.questionCount != 3 {
		t.Errorf("dispatch call = %+v, want gameID=%s question=q1 questionCount=3", call, testGameID)
	}
	if !slices.Equal(call.recipients, wantRecipients) {
		t.Errorf("dispatch recipients = %v, want %v", call.recipients, wantRecipients)
	}
	if got := engine.PlayerRecipientsRequestedFor(); len(got) != 1 || got[0] != testGameID {
		t.Errorf("PlayerRecipients requested for %v, want exactly one call for %s", got, testGameID)
	}
}

func TestNextQuestionDispatchesQuestionToPlayers(t *testing.T) {
	question := game.CurrentQuestion{ID: "q2", Position: 2, Type: "free_text", Text: "Capital?", TimeLimitSeconds: 30}
	wantRecipients := []string{"+972500000001"}
	engine := &stubControlEngine{
		nextQuestionSnapshot: game.Snapshot{GameID: testGameID, State: "question_open", QuestionCount: 3, CurrentQuestion: &question},
		playerRecipients:     wantRecipients,
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubQuestionDispatcher{done: make(chan struct{}, 1)}
	rec := httptest.NewRecorder()
	controlRouterWithDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/next-question", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /next-question = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	waitForSignal(t, dispatcher.done)

	calls := dispatcher.Calls()
	if len(calls) != 1 {
		t.Fatalf("DispatchQuestionOpened called %d times, want 1", len(calls))
	}
	call := calls[0]
	if call.gameID != testGameID || call.question.ID != "q2" || call.questionCount != 3 {
		t.Errorf("dispatch call = %+v, want gameID=%s question=q2 questionCount=3", call, testGameID)
	}
	if !slices.Equal(call.recipients, wantRecipients) {
		t.Errorf("dispatch recipients = %v, want %v", call.recipients, wantRecipients)
	}
}

func TestNextQuestionFinishingGameDoesNotDispatch(t *testing.T) {
	// The finishing branch: NextQuestion succeeds but the game is finished,
	// not question_open — dispatchQuestionOpened's guard trips on the state
	// check before it ever touches PlayerRecipients or the dispatcher, so
	// nothing async is ever spawned to wait for here.
	engine := &stubControlEngine{
		nextQuestionSnapshot: game.Snapshot{GameID: testGameID, State: "finished", CurrentQuestion: nil},
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubQuestionDispatcher{}
	rec := httptest.NewRecorder()
	controlRouterWithDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/next-question", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /next-question = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if calls := dispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchQuestionOpened called %d times, want 0 on the finishing branch", len(calls))
	}
	if got := engine.PlayerRecipientsRequestedFor(); len(got) != 0 {
		t.Errorf("PlayerRecipients called %d times, want 0 on the finishing branch", len(got))
	}
}

func TestStartGameDispatchSkippedOnRecipientsError(t *testing.T) {
	question := game.CurrentQuestion{ID: "q1", Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20}
	engine := &stubControlEngine{
		startGameSnapshot:    game.Snapshot{GameID: testGameID, State: "question_open", QuestionCount: 3, CurrentQuestion: &question},
		playerRecipientsErr:  errors.New("db down"),
		playerRecipientsDone: make(chan struct{}, 1),
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubQuestionDispatcher{}
	rec := httptest.NewRecorder()
	controlRouterWithDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/start", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /start = %d, want 200 even when recipient lookup fails (already-committed transition)", rec.Code)
	}
	waitForSignal(t, engine.playerRecipientsDone)

	if calls := dispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchQuestionOpened called %d times, want 0 when PlayerRecipients errors", len(calls))
	}
}

func TestOtherControlActionsNeverDispatch(t *testing.T) {
	// CloseQuestion/Reveal/StopGame don't accept a dispatcher param at all,
	// so dispatchQuestionOpened is never reached from these handlers — true
	// by construction. The snapshot below carries a non-nil CurrentQuestion,
	// matching what buildSnapshot really produces once
	// CurrentQuestionPosition > 0 regardless of state, so this test cannot
	// pass by accident of a nil-CurrentQuestion guard alone — it is
	// dispatchQuestionOpened's State == question_open check (and the fact
	// this handler never calls it) that actually protects this path.
	question := game.CurrentQuestion{ID: "q1", Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20}
	engine := &stubControlEngine{closeQuestionSnapshot: game.Snapshot{GameID: testGameID, State: "question_closed", CurrentQuestion: &question}}
	hub := &stubBroadcaster{}
	dispatcher := &stubQuestionDispatcher{}
	rec := httptest.NewRecorder()
	controlRouterWithDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/close-question", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /close-question = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if calls := dispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchQuestionOpened called %d times, want 0 for /close-question", len(calls))
	}
}
