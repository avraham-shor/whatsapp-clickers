package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
	"github.com/avraham-shor/whatsapp-clickers/internal/game"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
)

const displaySettingsPath = "/api/games/" + testGameID + "/display-settings"

// displaySnapshot is the engine's canned reply for the tests below:
// a lobby snapshot carrying the written reduced-motion value.
func displaySnapshot(reducedMotion bool) game.Snapshot {
	return game.Snapshot{
		GameID: testGameID, State: "lobby", JoinCode: "AB2CD3", PlatformNumber: "+972 50-000-0000",
		Participants:    []game.ParticipantSummary{},
		Leaderboard:     []game.LeaderboardEntry{},
		DisplaySettings: game.DisplaySettings{ReducedMotion: reducedMotion},
	}
}

func TestUpdateDisplaySettingsBroadcastsAndReturnsSnapshot(t *testing.T) {
	engine := &stubControlEngine{setDisplaySettingsSnapshot: displaySnapshot(true)}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPut, displaySettingsPath, `{"reducedMotion":true}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /display-settings = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		GameID          string `json:"gameId"`
		DisplaySettings struct {
			ReducedMotion bool `json:"reducedMotion"`
		} `json:"displaySettings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.GameID != testGameID || !body.DisplaySettings.ReducedMotion {
		t.Errorf("body = %+v, want the snapshot with displaySettings.reducedMotion true", body)
	}
	if got := engine.setDisplaySettingsRequestedFor; len(got) != 1 || got[0] != (setDisplaySettingsCall{testGameID, "org-1", true}) {
		t.Errorf("engine calls = %+v, want exactly one (%s, org-1, true)", got, testGameID)
	}
	if len(hub.calls) != 1 || hub.calls[0].gameID != testGameID {
		t.Fatalf("hub calls = %+v, want exactly one Broadcast for this game", hub.calls)
	}
	if !hub.calls[0].snapshot.DisplaySettings.ReducedMotion {
		t.Errorf("broadcast snapshot = %+v, want the updated settings", hub.calls[0].snapshot.DisplaySettings)
	}
}

// The pointer guard rejects ABSENCE, not the value: `{}` must never be
// read as "turn animations back on".
func TestUpdateDisplaySettingsMissingFieldReturns400(t *testing.T) {
	engine := &stubControlEngine{setDisplaySettingsSnapshot: displaySnapshot(false)}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPut, displaySettingsPath, `{}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT /display-settings with an empty body = %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "VALIDATION_FAILED" {
		t.Errorf("error code = %q, want VALIDATION_FAILED", code)
	}
	if got := engine.setDisplaySettingsRequestedFor; len(got) != 0 {
		t.Errorf("engine reached with an absent field: %+v", got)
	}
	if len(hub.calls) != 0 {
		t.Error("Broadcast called on a rejected request")
	}
}

// The other half of the pointer guard: false is a legitimate value and
// must reach the engine as one.
func TestUpdateDisplaySettingsFalseIsAccepted(t *testing.T) {
	engine := &stubControlEngine{setDisplaySettingsSnapshot: displaySnapshot(false)}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPut, displaySettingsPath, `{"reducedMotion":false}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /display-settings false = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if got := engine.setDisplaySettingsRequestedFor; len(got) != 1 || got[0].reducedMotion {
		t.Errorf("engine calls = %+v, want exactly one recording reducedMotion false", got)
	}
	if len(hub.calls) != 1 {
		t.Errorf("hub calls = %d, want exactly one Broadcast", len(hub.calls))
	}
}

func TestUpdateDisplaySettingsForeignOrMissingGameReturns404(t *testing.T) {
	engine := &stubControlEngine{setDisplaySettingsErr: store.ErrNotFound}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPut, displaySettingsPath, `{"reducedMotion":true}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("PUT /display-settings for a foreign game = %d, want 404 (body %s)", rec.Code, rec.Body)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
	if len(hub.calls) != 0 {
		t.Error("Broadcast called on a failed settings write")
	}
}

func TestUpdateDisplaySettingsMalformedGameIDReturns404(t *testing.T) {
	engine := &stubControlEngine{setDisplaySettingsSnapshot: displaySnapshot(true)}
	hub := &stubBroadcaster{}
	rec := httptest.NewRecorder()
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/not-a-uuid/display-settings", `{"reducedMotion":true}`))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("PUT /display-settings with a malformed id = %d, want 404 (body %s)", rec.Code, rec.Body)
	}
	if got := engine.setDisplaySettingsRequestedFor; len(got) != 0 {
		t.Errorf("malformed id reached the engine instead of short-circuiting: %+v", got)
	}
}

// A body the handler would otherwise accept, deliberately: with a bodyless
// request this passes whether requireOrganizer runs before or after the
// decode/pointer guard, so it would pin nothing. A valid body makes 401 the
// only answer that proves auth comes first. (Code review, 2026-08-09.)
//
// Written here rather than as an entry in games_test.go's
// TestGameMutationsWithoutSessionReturn401 (nil engine/hub there, so this
// route does not exist on that router) or control_test.go's
// controlActionCases table (POST-only, bodyless). Same shape as
// TestOpenLobbyWithoutSessionReturns401.
func TestUpdateDisplaySettingsWithoutSessionReturns401(t *testing.T) {
	engine := &stubControlEngine{}
	hub := &stubBroadcaster{}
	router := NewRouter(stubPinger{}, noAuth(), noGames(), testStatic(), nil, engine, hub, nil, nil, nil, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, displaySettingsPath, strings.NewReader(`{"reducedMotion":true}`)))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("PUT /display-settings without session = %d, want 401 (body %s)", rec.Code, rec.Body)
	}
	if got := engine.setDisplaySettingsRequestedFor; len(got) != 0 {
		t.Errorf("unauthenticated request reached the engine: %+v", got)
	}
}

// decodeJSON's behavior on this route, pinned rather than assumed: a
// wrong-typed field is a decode failure (INVALID_REQUEST), not the
// pointer guard's VALIDATION_FAILED, and it must not reach the engine as
// a zero value. (Code review, 2026-08-09.)
func TestUpdateDisplaySettingsMalformedBodyReturns400(t *testing.T) {
	for name, body := range map[string]string{
		"wrong type":  `{"reducedMotion":"yes"}`,
		"not json":    `{`,
		"json null":   `null`,
		"wrong shape": `[]`,
	} {
		engine := &stubControlEngine{setDisplaySettingsSnapshot: displaySnapshot(true)}
		hub := &stubBroadcaster{}
		rec := httptest.NewRecorder()
		controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPut, displaySettingsPath, body))

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: PUT /display-settings %s = %d, want 400 (body %s)", name, body, rec.Code, rec.Body)
		}
		if got := engine.setDisplaySettingsRequestedFor; len(got) != 0 {
			t.Errorf("%s: malformed body reached the engine: %+v", name, got)
		}
		if len(hub.calls) != 0 {
			t.Errorf("%s: Broadcast called on a rejected request", name)
		}
	}
}

// The route lives inside router.go's `engine != nil && hub != nil` guard.
// Each nil is exercised on its own: passing nil for both cannot tell
// "gated on engine" from "gated on hub", so it would not actually prove
// the `&&` a later refactor might loosen. (Code review, 2026-08-09.)
func TestUpdateDisplaySettingsRouteAbsentWithoutEngineOrHub(t *testing.T) {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	engine := &stubControlEngine{setDisplaySettingsSnapshot: displaySnapshot(true)}
	hub := &stubBroadcaster{}

	routers := map[string]http.Handler{
		"neither":    NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, nil, nil, nil, nil, nil, nil),
		"nil hub":    NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, engine, nil, nil, nil, nil, nil),
		"nil engine": NewRouter(stubPinger{}, svc, noGames(), testStatic(), nil, nil, hub, nil, nil, nil, nil),
	}
	for name, router := range routers {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authedRequest(http.MethodPut, displaySettingsPath, `{"reducedMotion":true}`))

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: PUT /display-settings = %d, want 404", name, rec.Code)
		}
	}
	if got := engine.setDisplaySettingsRequestedFor; len(got) != 0 {
		t.Errorf("a guarded-off route still reached the engine: %+v", got)
	}
	if len(hub.calls) != 0 {
		t.Errorf("a guarded-off route still broadcast: %+v", hub.calls)
	}
}

// orderRecordingBroadcaster captures how much of the response body had
// been written at the moment Broadcast fired.
type orderRecordingBroadcaster struct {
	stubBroadcaster
	rec                *httptest.ResponseRecorder
	bodyLenAtBroadcast int
}

func (b *orderRecordingBroadcaster) Broadcast(gameID string, snapshot game.Snapshot) {
	b.bodyLenAtBroadcast = b.rec.Body.Len()
	b.stubBroadcaster.Broadcast(gameID, snapshot)
}

// display.go's doc comment gives broadcast-before-response as the reason
// the handler is shaped the way it is ("so an already-open display renders
// at least as promptly as the organizer's own dashboard"); nothing
// asserted it, so reordering the two lines was a free change. (Code
// review, 2026-08-09.)
func TestUpdateDisplaySettingsBroadcastsBeforeWritingTheResponse(t *testing.T) {
	engine := &stubControlEngine{setDisplaySettingsSnapshot: displaySnapshot(true)}
	rec := httptest.NewRecorder()
	hub := &orderRecordingBroadcaster{rec: rec}
	controlRouter(engine, hub).ServeHTTP(rec, authedRequest(http.MethodPut, displaySettingsPath, `{"reducedMotion":true}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /display-settings = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if len(hub.calls) != 1 {
		t.Fatalf("hub calls = %d, want exactly one Broadcast", len(hub.calls))
	}
	if hub.bodyLenAtBroadcast != 0 {
		t.Errorf("response body already held %d bytes when Broadcast fired, want the broadcast first", hub.bodyLenAtBroadcast)
	}
	if rec.Body.Len() == 0 {
		t.Error("response body is empty, so the ordering assertion above proves nothing")
	}
}
