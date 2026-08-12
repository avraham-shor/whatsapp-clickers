package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	revealPosition     int32
	revealErr          error
	revealRequestedFor [][2]string

	// Unguarded like its neighbours: handleShowLeaderboard spawns no
	// dispatch goroutine at all (story 4.5 — the Leaderboard is quiet on
	// WhatsApp), so nothing outside the request goroutine touches these.
	showLeaderboardSnapshot     game.Snapshot
	showLeaderboardErr          error
	showLeaderboardRequestedFor [][2]string

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

	// resultsForRevealedQuestionMu guards the fields below, same reasoning
	// as playerRecipientsMu — dispatchAnswerRevealed also runs in a
	// goroutine spawned after the HTTP response is written.
	resultsForRevealedQuestionMu           sync.Mutex
	resultsForRevealedQuestionResult       game.RevealedQuestionResults
	resultsForRevealedQuestionErr          error
	resultsForRevealedQuestionRequestedFor [][2]string // {gameID, organizerID}
	resultsForRevealedQuestionPosition     int32
	resultsForRevealedQuestionDone         chan struct{}

	// resultsForFinishedGameMu guards the fields below, same reasoning
	// again — dispatchGameFinished runs in a goroutine spawned after the
	// HTTP response is written (story 3.9).
	resultsForFinishedGameMu           sync.Mutex
	resultsForFinishedGameResult       game.FinalResults
	resultsForFinishedGameErr          error
	resultsForFinishedGameRequestedFor []string
	resultsForFinishedGameDone         chan struct{}

	// Unguarded like the six transitions above, and for the same reason:
	// SetDisplaySettings runs entirely in the request goroutine and spawns
	// no post-response dispatch. The mutexes above exist only for the
	// methods a dispatch goroutine can reach (story 4.1).
	setDisplaySettingsSnapshot     game.Snapshot
	setDisplaySettingsErr          error
	setDisplaySettingsRequestedFor []setDisplaySettingsCall
}

// setDisplaySettingsCall records one SetDisplaySettings call. Unlike the
// [2]string the transitions above record, it carries the written value
// too, so a test can prove `false` reached the engine as a deliberate
// value rather than as a decoded absence.
type setDisplaySettingsCall struct {
	gameID        string
	organizerID   string
	reducedMotion bool
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

// Reveal's second return is the just-revealed position, which
// dispatchAnswerRevealed binds to instead of the snapshot (code review,
// story 3.8). revealPosition defaults to 0, so tests that exercise result
// dispatch must set it explicitly — 0 is the "not set" value the dispatch
// guard rejects, exactly as on Reveal's own error paths.
func (s *stubControlEngine) Reveal(ctx context.Context, gameID, organizerID string) (game.Snapshot, int32, error) {
	s.revealRequestedFor = append(s.revealRequestedFor, [2]string{gameID, organizerID})
	return s.revealSnapshot, s.revealPosition, s.revealErr
}

func (s *stubControlEngine) ShowLeaderboard(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.showLeaderboardRequestedFor = append(s.showLeaderboardRequestedFor, [2]string{gameID, organizerID})
	return s.showLeaderboardSnapshot, s.showLeaderboardErr
}

func (s *stubControlEngine) NextQuestion(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.nextQuestionRequestedFor = append(s.nextQuestionRequestedFor, [2]string{gameID, organizerID})
	return s.nextQuestionSnapshot, s.nextQuestionErr
}

func (s *stubControlEngine) StopGame(ctx context.Context, gameID, organizerID string) (game.Snapshot, error) {
	s.stopGameRequestedFor = append(s.stopGameRequestedFor, [2]string{gameID, organizerID})
	return s.stopGameSnapshot, s.stopGameErr
}

func (s *stubControlEngine) SetDisplaySettings(ctx context.Context, gameID, organizerID string, reducedMotion bool) (game.Snapshot, error) {
	s.setDisplaySettingsRequestedFor = append(s.setDisplaySettingsRequestedFor, setDisplaySettingsCall{gameID, organizerID, reducedMotion})
	return s.setDisplaySettingsSnapshot, s.setDisplaySettingsErr
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

// ResultsForRevealedQuestionRequestedFor and
// ResultsForRevealedQuestionPosition copy under the mutex, same as
// PlayerRecipientsRequestedFor — these fields are written from the
// post-response dispatch goroutine, so tests must never read them
// directly (code review, story 3.8).
func (s *stubControlEngine) ResultsForRevealedQuestionRequestedFor() [][2]string {
	s.resultsForRevealedQuestionMu.Lock()
	defer s.resultsForRevealedQuestionMu.Unlock()
	return append([][2]string(nil), s.resultsForRevealedQuestionRequestedFor...)
}

func (s *stubControlEngine) ResultsForRevealedQuestionPosition() int32 {
	s.resultsForRevealedQuestionMu.Lock()
	defer s.resultsForRevealedQuestionMu.Unlock()
	return s.resultsForRevealedQuestionPosition
}

func (s *stubControlEngine) ResultsForRevealedQuestion(ctx context.Context, gameID, organizerID string, position int32) (game.RevealedQuestionResults, error) {
	s.resultsForRevealedQuestionMu.Lock()
	s.resultsForRevealedQuestionRequestedFor = append(s.resultsForRevealedQuestionRequestedFor, [2]string{gameID, organizerID})
	s.resultsForRevealedQuestionPosition = position
	result, err := s.resultsForRevealedQuestionResult, s.resultsForRevealedQuestionErr
	s.resultsForRevealedQuestionMu.Unlock()
	if s.resultsForRevealedQuestionDone != nil {
		s.resultsForRevealedQuestionDone <- struct{}{}
	}
	return result, err
}

func (s *stubControlEngine) ResultsForFinishedGame(ctx context.Context, gameID string) (game.FinalResults, error) {
	s.resultsForFinishedGameMu.Lock()
	s.resultsForFinishedGameRequestedFor = append(s.resultsForFinishedGameRequestedFor, gameID)
	result, err := s.resultsForFinishedGameResult, s.resultsForFinishedGameErr
	s.resultsForFinishedGameMu.Unlock()
	if s.resultsForFinishedGameDone != nil {
		s.resultsForFinishedGameDone <- struct{}{}
	}
	return result, err
}

// ResultsForFinishedGameRequestedFor returns a thread-safe snapshot of
// every gameID ResultsForFinishedGame was called with.
func (s *stubControlEngine) ResultsForFinishedGameRequestedFor() []string {
	s.resultsForFinishedGameMu.Lock()
	defer s.resultsForFinishedGameMu.Unlock()
	return append([]string(nil), s.resultsForFinishedGameRequestedFor...)
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

// stubResultDispatcher implements ResultDispatcher and records every
// DispatchAnswerRevealed call, mirroring stubQuestionDispatcher's shape —
// dispatchAnswerRevealed also runs in a goroutine spawned after the HTTP
// response is written (see control.go).
type stubResultDispatcher struct {
	mu    sync.Mutex
	calls []resultDispatchCall
	done  chan struct{}
}

type resultDispatchCall struct {
	gameID            string
	results           []game.PersonalResult
	correctAnswerText string
	isLastQuestion    bool
}

func (s *stubResultDispatcher) DispatchAnswerRevealed(gameID string, results []game.PersonalResult, correctAnswerText string, isLastQuestion bool) {
	s.mu.Lock()
	s.calls = append(s.calls, resultDispatchCall{gameID, results, correctAnswerText, isLastQuestion})
	s.mu.Unlock()
	if s.done != nil {
		s.done <- struct{}{}
	}
}

// Calls returns a thread-safe snapshot of every recorded call.
func (s *stubResultDispatcher) Calls() []resultDispatchCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]resultDispatchCall(nil), s.calls...)
}

// stubFinalDispatcher implements FinalDispatcher and records every
// DispatchGameFinished call, mirroring stubResultDispatcher's shape —
// dispatchGameFinished also runs in a goroutine spawned after the HTTP
// response is written (see control.go, story 3.9).
type stubFinalDispatcher struct {
	mu    sync.Mutex
	calls []finalDispatchCall
	done  chan struct{}
}

type finalDispatchCall struct {
	gameID  string
	results game.FinalResults
}

func (s *stubFinalDispatcher) DispatchGameFinished(gameID string, results game.FinalResults) {
	s.mu.Lock()
	s.calls = append(s.calls, finalDispatchCall{gameID, results})
	s.mu.Unlock()
	if s.done != nil {
		s.done <- struct{}{}
	}
}

// Calls returns a thread-safe snapshot of every recorded call.
func (s *stubFinalDispatcher) Calls() []finalDispatchCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]finalDispatchCall(nil), s.calls...)
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
// given ControlEngine/SnapshotBroadcaster stubs. No QuestionDispatcher/
// ResultDispatcher — none of this file's existing tests exercise WhatsApp
// dispatch.
func controlRouter(engine ControlEngine, hub SnapshotBroadcaster) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil, nil, nil, nil)
}

// controlRouterWithDispatcher is controlRouter plus a real QuestionDispatcher
// — used only by the question-dispatch tests below.
func controlRouterWithDispatcher(engine ControlEngine, hub SnapshotBroadcaster, dispatcher QuestionDispatcher) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil, dispatcher, nil, nil)
}

// controlRouterWithResultDispatcher is controlRouter plus a real
// ResultDispatcher — used only by the result-dispatch tests below (story
// 3.8).
func controlRouterWithResultDispatcher(engine ControlEngine, hub SnapshotBroadcaster, resultDispatcher ResultDispatcher) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil, nil, resultDispatcher, nil)
}

// controlRouterWithFinalDispatcher is controlRouter plus a real
// FinalDispatcher — used only by the final-results-dispatch tests below
// (story 3.9).
func controlRouterWithFinalDispatcher(engine ControlEngine, hub SnapshotBroadcaster, finalDispatcher FinalDispatcher) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil, nil, nil, finalDispatcher)
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
	router := NewRouter(stubPinger{}, noAuth(), noGames(), testStatic(), nil, engine, hub, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/games/"+testGameID+"/open-lobby", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /open-lobby without session = %d, want 401", rec.Code)
	}
	if len(engine.openLobbyRequestedFor) != 0 {
		t.Error("unauthenticated request reached the engine")
	}
}

// controlActionCase describes one live-control action past open-lobby, so
// the success/error/404/401 suite below (mirroring the TestOpenLobby* suite
// above) runs once per action instead of being duplicated by hand. Five at
// story 3.1; six since story 4.5 added show-leaderboard, the transition 3.1
// deliberately left out.
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
		// Shares NextQuestion's error and code on purpose: entering the
		// Leaderboard and skipping it are the same precondition (the game
		// must be revealed), so story 4.5 added no seventh error type.
		name:         "ShowLeaderboard",
		path:         "/show-leaderboard",
		domainErr:    game.ErrNotRevealed,
		domainCode:   "GAME_NOT_REVEALED",
		setSnapshot:  func(e *stubControlEngine, s game.Snapshot) { e.showLeaderboardSnapshot = s },
		setErr:       func(e *stubControlEngine, err error) { e.showLeaderboardErr = err },
		requestedFor: func(e *stubControlEngine) [][2]string { return e.showLeaderboardRequestedFor },
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
			router := NewRouter(stubPinger{}, noAuth(), noGames(), testStatic(), nil, engine, hub, nil, nil, nil, nil)
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
	// CloseQuestion/StopGame don't accept a QuestionDispatcher param at all
	// (Reveal accepts a different one, ResultDispatcher — see the "Result
	// dispatch" block below; and as of story 3.9 StopGame accepts a
	// FinalDispatcher and NextQuestion accepts one in addition to its
	// QuestionDispatcher — see "Final results dispatch"), so
	// dispatchQuestionOpened is never
	// reached from these handlers — true by construction. The snapshot below
	// carries a non-nil CurrentQuestion,
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

// --- Result dispatch (story 3.8) ---

func TestRevealDispatchesResultsToResultDispatcher(t *testing.T) {
	question := game.CurrentQuestion{ID: "q1", Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20}
	wantResults := []game.PersonalResult{
		{Phone: "+972500000001", IsCorrect: true, BasePoints: 100, BonusPoints: 50, Rank: 1},
	}
	engine := &stubControlEngine{
		revealSnapshot: game.Snapshot{GameID: testGameID, State: "revealed", CurrentQuestion: &question},
		revealPosition: 1,
		resultsForRevealedQuestionResult: game.RevealedQuestionResults{
			Results: wantResults, CorrectAnswer: "2", IsLastQuestion: false,
		},
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubResultDispatcher{done: make(chan struct{}, 1)}
	rec := httptest.NewRecorder()
	controlRouterWithResultDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/reveal", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /reveal = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	waitForSignal(t, dispatcher.done)

	calls := dispatcher.Calls()
	if len(calls) != 1 {
		t.Fatalf("DispatchAnswerRevealed called %d times, want 1", len(calls))
	}
	call := calls[0]
	if call.gameID != testGameID || call.correctAnswerText != "2" || call.isLastQuestion || len(call.results) != 1 || call.results[0] != wantResults[0] {
		t.Errorf("dispatch call = %+v, want gameID=%s correctAnswerText=2 isLastQuestion=false results=%+v", call, testGameID, wantResults)
	}
	if got := engine.ResultsForRevealedQuestionPosition(); got != 1 {
		t.Errorf("ResultsForRevealedQuestion called with position %d, want 1 (Reveal's own returned position)", got)
	}
}

// The dispatch binds to Reveal's returned position, not to the snapshot,
// so a degraded snapshot (emptySnapshot: State "revealed", nil
// CurrentQuestion — what snapshotAfterCommit returns when buildSnapshot
// fails) must still deliver everyone's results. Before the story 3.8 code
// review this input silently dropped the entire fan-out, permanently,
// since Reveal refuses to run again from the revealed state.
func TestRevealDispatchesResultsWhenSnapshotDegradedToNilCurrentQuestion(t *testing.T) {
	wantResults := []game.PersonalResult{
		{Phone: "+972500000001", IsCorrect: false, Rank: 4},
	}
	engine := &stubControlEngine{
		revealSnapshot: game.Snapshot{GameID: testGameID, State: "revealed", CurrentQuestion: nil},
		revealPosition: 3,
		resultsForRevealedQuestionResult: game.RevealedQuestionResults{
			Results: wantResults, CorrectAnswer: "ירושלים", IsLastQuestion: true,
		},
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubResultDispatcher{done: make(chan struct{}, 1)}
	rec := httptest.NewRecorder()
	controlRouterWithResultDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/reveal", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /reveal = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	waitForSignal(t, dispatcher.done)

	calls := dispatcher.Calls()
	if len(calls) != 1 {
		t.Fatalf("DispatchAnswerRevealed called %d times, want 1 even with a degraded snapshot", len(calls))
	}
	if got := engine.ResultsForRevealedQuestionPosition(); got != 3 {
		t.Errorf("ResultsForRevealedQuestion called with position %d, want 3 (Reveal's returned position, not the nil snapshot's)", got)
	}
}

// position 0 is what Reveal returns on its error paths; the handler
// returns before dispatching in that case, but the guard is the last line
// of defence against dispatching results for a question nobody identified.
func TestRevealDispatchSkippedWhenPositionUnset(t *testing.T) {
	engine := &stubControlEngine{
		revealSnapshot: game.Snapshot{GameID: testGameID, State: "revealed", CurrentQuestion: nil},
		revealPosition: 0,
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubResultDispatcher{}
	rec := httptest.NewRecorder()
	controlRouterWithResultDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/reveal", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /reveal = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if got := engine.ResultsForRevealedQuestionRequestedFor(); len(got) != 0 {
		t.Errorf("ResultsForRevealedQuestion called %d times, want 0 when the revealed position is unset", len(got))
	}
	if calls := dispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchAnswerRevealed called %d times, want 0 when the revealed position is unset", len(calls))
	}
}

func TestRevealDispatchSkippedOnResultsError(t *testing.T) {
	question := game.CurrentQuestion{ID: "q1", Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20}
	engine := &stubControlEngine{
		revealSnapshot:                 game.Snapshot{GameID: testGameID, State: "revealed", CurrentQuestion: &question},
		revealPosition:                 1,
		resultsForRevealedQuestionErr:  errors.New("db down"),
		resultsForRevealedQuestionDone: make(chan struct{}, 1),
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubResultDispatcher{}
	rec := httptest.NewRecorder()
	controlRouterWithResultDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/reveal", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /reveal = %d, want 200 even when result resolution fails (already-committed transition)", rec.Code)
	}
	waitForSignal(t, engine.resultsForRevealedQuestionDone)

	if calls := dispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchAnswerRevealed called %d times, want 0 when ResultsForRevealedQuestion errors", len(calls))
	}
}

func TestOtherControlActionsNeverDispatchResults(t *testing.T) {
	// CloseQuestion/StartGame/NextQuestion/StopGame don't accept a
	// ResultDispatcher param at all — true by construction, since only
	// handleReveal does.
	question := game.CurrentQuestion{ID: "q1", Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20}
	engine := &stubControlEngine{closeQuestionSnapshot: game.Snapshot{GameID: testGameID, State: "question_closed", CurrentQuestion: &question}}
	hub := &stubBroadcaster{}
	dispatcher := &stubResultDispatcher{}
	rec := httptest.NewRecorder()
	controlRouterWithResultDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/close-question", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /close-question = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if calls := dispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchAnswerRevealed called %d times, want 0 for /close-question", len(calls))
	}
	if got := engine.ResultsForRevealedQuestionRequestedFor(); len(got) != 0 {
		t.Errorf("ResultsForRevealedQuestion called %d times, want 0 for /close-question", len(got))
	}
}

func TestShowLeaderboardDispatchesNoWhatsAppTrafficAtAll(t *testing.T) {
	// The Leaderboard is the only transition that is silent on WhatsApp
	// (EXPERIENCE.md's State Patterns table: "— (quiet)"). handleShowLeaderboard
	// takes no dispatcher parameter, so this is true by construction — but
	// "by construction" is exactly what a later refactor breaks quietly, and
	// the failure mode is a whole room's phones buzzing between questions.
	// All three dispatchers are wired in and all three must stay silent.
	question := game.CurrentQuestion{ID: "q1", Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20}
	engine := &stubControlEngine{
		showLeaderboardSnapshot: game.Snapshot{GameID: testGameID, State: "leaderboard", CurrentQuestion: &question},
		playerRecipients:        []string{"+972500000001"},
	}
	hub := &stubBroadcaster{}
	// Buffered signal channels on all three, because the assertion below is a
	// NEGATIVE one and needs something to wait on. Buffered so a regression
	// that did dispatch records its call and returns rather than blocking on
	// an unread channel.
	questionDispatcher := &stubQuestionDispatcher{done: make(chan struct{}, 1)}
	resultDispatcher := &stubResultDispatcher{done: make(chan struct{}, 1)}
	finalDispatcher := &stubFinalDispatcher{done: make(chan struct{}, 1)}
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	router := NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil, questionDispatcher, resultDispatcher, finalDispatcher)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/show-leaderboard", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /show-leaderboard = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if len(hub.calls) != 1 {
		t.Errorf("Broadcast called %d times, want exactly 1 — the room's screen is the only thing this transition talks to", len(hub.calls))
	}

	// Every sibling handler dispatches from a goroutine spawned AFTER the
	// response is written (control.go: handleStartGame, handleReveal,
	// handleNextQuestion, handleStopGame). Reading the counters straight after
	// ServeHTTP therefore proves nothing: a refactor that added a `go
	// dispatch…` here would almost never have been scheduled by then, so the
	// counters would read 0 and this test would pass over the exact regression
	// it exists to catch. Wait for a signal that must never arrive instead,
	// and only then read them. (Code review, 2026-08-12.)
	select {
	case <-questionDispatcher.done:
		t.Fatal("DispatchQuestionOpened fired — EXPERIENCE.md gives the Leaderboard row \"— (quiet)\" on WhatsApp")
	case <-resultDispatcher.done:
		t.Fatal("DispatchAnswerRevealed fired — EXPERIENCE.md gives the Leaderboard row \"— (quiet)\" on WhatsApp")
	case <-finalDispatcher.done:
		t.Fatal("DispatchGameFinished fired — EXPERIENCE.md gives the Leaderboard row \"— (quiet)\" on WhatsApp")
	case <-time.After(100 * time.Millisecond):
	}

	if calls := questionDispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchQuestionOpened called %d times, want 0", len(calls))
	}
	if calls := resultDispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchAnswerRevealed called %d times, want 0", len(calls))
	}
	if calls := finalDispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchGameFinished called %d times, want 0", len(calls))
	}
	if got := engine.PlayerRecipientsRequestedFor(); len(got) != 0 {
		t.Errorf("PlayerRecipients called %d times, want 0 — nothing here needs a recipient list", len(got))
	}
}

// --- Final results dispatch (story 3.9) ---

// finalResultsFixture is the resolved game-end data set the stub engine
// hands back — a sole winner plus one other player, enough to prove the
// dispatcher receives exactly what ResultsForFinishedGame returned.
func finalResultsFixture() game.FinalResults {
	return game.FinalResults{
		Recipients: []game.FinalRecipient{
			{Phone: "+972500000001", IsWinner: true, Rank: 1, Score: 300},
			{Phone: "+972500000002", Rank: 2, Score: 200},
		},
		WinnerNames: []string{"David Cohen"},
		WinnerScore: 300,
	}
}

func assertOneFinalDispatch(t *testing.T, dispatcher *stubFinalDispatcher, want game.FinalResults) {
	t.Helper()
	calls := dispatcher.Calls()
	if len(calls) != 1 {
		t.Fatalf("DispatchGameFinished called %d times, want 1", len(calls))
	}
	if calls[0].gameID != testGameID {
		t.Errorf("dispatch gameID = %q, want %q", calls[0].gameID, testGameID)
	}
	if !reflect.DeepEqual(calls[0].results, want) {
		t.Errorf("dispatch results = %+v, want %+v", calls[0].results, want)
	}
}

func TestNextQuestionPastLastQuestionDispatchesFinalResults(t *testing.T) {
	engine := &stubControlEngine{
		nextQuestionSnapshot:         game.Snapshot{GameID: testGameID, State: "finished", CurrentQuestion: nil},
		resultsForFinishedGameResult: finalResultsFixture(),
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubFinalDispatcher{done: make(chan struct{}, 1)}
	rec := httptest.NewRecorder()
	controlRouterWithFinalDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/next-question", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /next-question = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	waitForSignal(t, dispatcher.done)

	assertOneFinalDispatch(t, dispatcher, finalResultsFixture())
	if got := engine.ResultsForFinishedGameRequestedFor(); len(got) != 1 || got[0] != testGameID {
		t.Errorf("ResultsForFinishedGame requested for %v, want exactly [%s]", got, testGameID)
	}
}

// TestNextQuestionToAnotherQuestionDoesNotDispatchFinalResults pins that
// the two goroutines handleNextQuestion spawns are mutually exclusive:
// on a question_open outcome the question dispatch runs and the
// final-results dispatch's State guard trips. Wires BOTH dispatchers
// inline (the controlRouter* helpers each wire only one) so the positive
// half — the question dispatch still firing, unchanged — is asserted
// alongside the negative half.
func TestNextQuestionToAnotherQuestionDoesNotDispatchFinalResults(t *testing.T) {
	question := game.CurrentQuestion{ID: "q2", Position: 2, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20}
	engine := &stubControlEngine{
		nextQuestionSnapshot: game.Snapshot{GameID: testGameID, State: "question_open", QuestionCount: 3, CurrentQuestion: &question},
		playerRecipients:     []string{"+972500000001"},
	}
	hub := &stubBroadcaster{}
	questionDispatcher := &stubQuestionDispatcher{done: make(chan struct{}, 1)}
	finalDispatcher := &stubFinalDispatcher{}
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	router := NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, hub, nil, questionDispatcher, nil, finalDispatcher)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/next-question", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /next-question = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	waitForSignal(t, questionDispatcher.done)

	if calls := questionDispatcher.Calls(); len(calls) != 1 {
		t.Errorf("DispatchQuestionOpened called %d times, want 1 — the question path is unchanged", len(calls))
	}
	if calls := finalDispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchGameFinished called %d times, want 0 when the game is not finished", len(calls))
	}
	if got := engine.ResultsForFinishedGameRequestedFor(); len(got) != 0 {
		t.Errorf("ResultsForFinishedGame called %d times, want 0 when the game is not finished", len(got))
	}
}

// TestStopGameDispatchesFinalResults is the AC-1 case the natural
// end-of-game path does not cover: an Organizer aborting a live round
// owes the room the same closure (see the story's design notes).
func TestStopGameDispatchesFinalResults(t *testing.T) {
	engine := &stubControlEngine{
		stopGameSnapshot:             game.Snapshot{GameID: testGameID, State: "finished", CurrentQuestion: nil},
		resultsForFinishedGameResult: finalResultsFixture(),
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubFinalDispatcher{done: make(chan struct{}, 1)}
	rec := httptest.NewRecorder()
	controlRouterWithFinalDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/stop", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /stop = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	waitForSignal(t, dispatcher.done)

	assertOneFinalDispatch(t, dispatcher, finalResultsFixture())
	if got := engine.ResultsForFinishedGameRequestedFor(); len(got) != 1 || got[0] != testGameID {
		t.Errorf("ResultsForFinishedGame requested for %v, want exactly [%s]", got, testGameID)
	}
}

// TestFinalResultsDispatchSurvivesDegradedSnapshot pins the design note
// that State is the ONLY snapshot field this path reads. emptySnapshot
// (what snapshotAfterCommit falls back to when buildSnapshot fails)
// carries State plus nothing else — story 3.8's original design read
// CurrentQuestion off the snapshot and silently lost its entire fan-out
// on exactly this input. Here the whole data set is re-resolved from the
// DB by ResultsForFinishedGame(gameID), so the failure mode is
// structurally impossible; this test keeps it that way.
func TestFinalResultsDispatchSurvivesDegradedSnapshot(t *testing.T) {
	engine := &stubControlEngine{
		stopGameSnapshot:             game.Snapshot{GameID: testGameID, State: "finished", CurrentQuestion: nil, Participants: []game.ParticipantSummary{}},
		resultsForFinishedGameResult: finalResultsFixture(),
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubFinalDispatcher{done: make(chan struct{}, 1)}
	rec := httptest.NewRecorder()
	controlRouterWithFinalDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/stop", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /stop = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	waitForSignal(t, dispatcher.done)

	assertOneFinalDispatch(t, dispatcher, finalResultsFixture())
}

func TestFinalResultsDispatchSkippedOnEngineError(t *testing.T) {
	engine := &stubControlEngine{
		stopGameSnapshot:           game.Snapshot{GameID: testGameID, State: "finished"},
		resultsForFinishedGameErr:  errors.New("db down"),
		resultsForFinishedGameDone: make(chan struct{}, 1),
	}
	hub := &stubBroadcaster{}
	dispatcher := &stubFinalDispatcher{}
	rec := httptest.NewRecorder()
	controlRouterWithFinalDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/stop", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /stop = %d, want 200 even when final-results resolution fails (already-committed transition)", rec.Code)
	}
	waitForSignal(t, engine.resultsForFinishedGameDone)

	if calls := dispatcher.Calls(); len(calls) != 0 {
		t.Errorf("DispatchGameFinished called %d times, want 0 when ResultsForFinishedGame errors", len(calls))
	}
}

func TestOtherControlActionsNeverDispatchFinalResults(t *testing.T) {
	// StartGame/CloseQuestion/Reveal don't accept a FinalDispatcher param
	// at all — true by construction, since only handleNextQuestion and
	// handleStopGame do. Each snapshot below carries a live, non-finished
	// state, so nothing here could trip dispatchGameFinished's guard even
	// if one of these handlers were wired to it by mistake.
	question := game.CurrentQuestion{ID: "q1", Position: 1, Type: "mcq", Text: "1+1?", Options: []string{"1", "2", "3", "4"}, TimeLimitSeconds: 20}
	cases := []struct {
		route       string
		setSnapshot func(*stubControlEngine)
	}{
		{"start", func(e *stubControlEngine) {
			e.startGameSnapshot = game.Snapshot{GameID: testGameID, State: "question_open", CurrentQuestion: &question}
		}},
		{"close-question", func(e *stubControlEngine) {
			e.closeQuestionSnapshot = game.Snapshot{GameID: testGameID, State: "question_closed", CurrentQuestion: &question}
		}},
		{"reveal", func(e *stubControlEngine) {
			e.revealSnapshot = game.Snapshot{GameID: testGameID, State: "revealed", CurrentQuestion: &question}
			e.revealPosition = 1
		}},
	}
	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			engine := &stubControlEngine{}
			tc.setSnapshot(engine)
			hub := &stubBroadcaster{}
			dispatcher := &stubFinalDispatcher{}
			rec := httptest.NewRecorder()
			controlRouterWithFinalDispatcher(engine, hub, dispatcher).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games/"+testGameID+"/"+tc.route, ""))

			if rec.Code != http.StatusOK {
				t.Fatalf("POST /%s = %d, want 200 (body %s)", tc.route, rec.Code, rec.Body)
			}
			if calls := dispatcher.Calls(); len(calls) != 0 {
				t.Errorf("DispatchGameFinished called %d times, want 0 for /%s", len(calls), tc.route)
			}
			// Read through the copy-under-lock accessor, never the raw
			// field — this method runs from a post-response goroutine.
			if got := engine.ResultsForFinishedGameRequestedFor(); len(got) != 0 {
				t.Errorf("ResultsForFinishedGame called %d times, want 0 for /%s", len(got), tc.route)
			}
		})
	}
}
