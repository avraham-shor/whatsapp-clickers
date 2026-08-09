package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
)

// resultsBody is the decoded post-game summary; assertions that must
// distinguish [] from null read rec.Body directly instead (see
// TestGameResultsEmptyGameSerializesEmptyArrays).
type resultsBody struct {
	GameID      string `json:"gameId"`
	Title       string `json:"title"`
	State       string `json:"state"`
	PlayerCount int    `json:"playerCount"`
	Leaderboard []struct {
		ParticipantID string `json:"participantId"`
		DisplayName   string `json:"displayName"`
		Score         int32  `json:"score"`
		Rank          int    `json:"rank"`
	} `json:"leaderboard"`
	Questions []struct {
		ID            string `json:"id"`
		Position      int32  `json:"position"`
		Type          string `json:"type"`
		Text          string `json:"text"`
		AnsweredCount int32  `json:"answeredCount"`
		CorrectCount  int32  `json:"correctCount"`
	} `json:"questions"`
}

func getResults(t *testing.T, games GameStore) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games/"+testGameID+"/results", ""))
	return rec
}

func decodeResults(t *testing.T, rec *httptest.ResponseRecorder) resultsBody {
	t.Helper()
	var body resultsBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("results response is not valid JSON: %v (body %s)", err, rec.Body)
	}
	return body
}

func TestGameResultsReturnsRankedLeaderboardAndQuestionStats(t *testing.T) {
	games := finishedGame()
	games.leaderboard = []store.ParticipantScore{
		{ParticipantID: "p-1", DisplayName: "דנה", Score: 120},
		{ParticipantID: "p-2", DisplayName: "יוסי", Score: 300},
		{ParticipantID: "p-3", DisplayName: "רות", Score: 200},
	}
	games.questionStats = []store.QuestionResponseStats{
		{QuestionID: "q-1", Position: 1, Type: "mcq", Text: "מה בירת צרפת?", AnsweredCount: 3, CorrectCount: 2},
		{QuestionID: "q-2", Position: 2, Type: "free_text", Text: "מי כתב את התנ״ך?", AnsweredCount: 1, CorrectCount: 1},
	}

	rec := getResults(t, games)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /results = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	body := decodeResults(t, rec)

	// Title is asserted, not merely declared on resultsBody: it is what names
	// the game on the routed page an organizer reaches cold from a bookmark,
	// so a dropped or wrong title is a real defect and not a cosmetic one.
	if body.GameID != testGameID || body.State != "finished" || body.Title != "ערב טריוויה" {
		t.Errorf("results = %+v, want the finished game's id, state and title", body)
	}
	if body.PlayerCount != 3 {
		t.Errorf("playerCount = %d, want 3 (one row per player-role participant)", body.PlayerCount)
	}
	if len(body.Leaderboard) != 3 {
		t.Fatalf("leaderboard has %d entries, want 3", len(body.Leaderboard))
	}
	// Score-descending with distinct ranks; the store order (120/300/200)
	// is deliberately not the wire order — RankLeaderboard sorts.
	// displayName is included because it is the one leaderboard field the
	// organizer actually reads on screen — dropping it would otherwise leave
	// every test in this file green.
	for i, want := range []struct {
		id    string
		name  string
		score int32
		rank  int
	}{{"p-2", "יוסי", 300, 1}, {"p-3", "רות", 200, 2}, {"p-1", "דנה", 120, 3}} {
		got := body.Leaderboard[i]
		if got.ParticipantID != want.id || got.DisplayName != want.name ||
			got.Score != want.score || got.Rank != want.rank {
			t.Errorf("leaderboard[%d] = %+v, want %s (%s) score %d rank %d",
				i, got, want.id, want.name, want.score, want.rank)
		}
	}

	if len(body.Questions) != 2 {
		t.Fatalf("questions has %d entries, want 2", len(body.Questions))
	}
	q1, q2 := body.Questions[0], body.Questions[1]
	if q1.ID != "q-1" || q1.Position != 1 || q1.Type != "mcq" || q1.AnsweredCount != 3 || q1.CorrectCount != 2 {
		t.Errorf("questions[0] = %+v, want q-1 pos 1 mcq 3 answered 2 correct", q1)
	}
	if q2.ID != "q-2" || q2.Position != 2 || q2.Type != "free_text" || q2.AnsweredCount != 1 || q2.CorrectCount != 1 {
		t.Errorf("questions[1] = %+v, want q-2 pos 2 free_text 1 answered 1 correct", q2)
	}

	if len(games.gotGame) != 1 || games.gotGame[0] != [2]string{testGameID, "org-1"} {
		t.Errorf("game lookup = %v, want exactly one scoped to org-1", games.gotGame)
	}
}

func TestGameResultsSharedRanksOnTie(t *testing.T) {
	// Standard competition ranks through the wire: 1,1,3 — never 1,1,2.
	games := finishedGame()
	games.leaderboard = []store.ParticipantScore{
		{ParticipantID: "p-1", DisplayName: "דנה", Score: 300},
		{ParticipantID: "p-2", DisplayName: "יוסי", Score: 300},
		{ParticipantID: "p-3", DisplayName: "רות", Score: 100},
	}

	rec := getResults(t, games)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /results = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	body := decodeResults(t, rec)

	if len(body.Leaderboard) != 3 {
		t.Fatalf("leaderboard has %d entries, want 3", len(body.Leaderboard))
	}
	gotRanks := [3]int{body.Leaderboard[0].Rank, body.Leaderboard[1].Rank, body.Leaderboard[2].Rank}
	if gotRanks != [3]int{1, 1, 3} {
		t.Errorf("ranks = %v, want [1 1 3] (tied top shares rank 1, next skips to 3)", gotRanks)
	}
	// The tie breaks by store order (join order), which RankLeaderboard's
	// stable sort preserves.
	if body.Leaderboard[0].ParticipantID != "p-1" || body.Leaderboard[1].ParticipantID != "p-2" {
		t.Errorf("tied order = %s,%s, want p-1,p-2 (join order preserved)",
			body.Leaderboard[0].ParticipantID, body.Leaderboard[1].ParticipantID)
	}
}

func TestGameResultsNotFinishedReturns409(t *testing.T) {
	// The epic's AC is "Given a finished Game": every other state answers
	// 409 rather than a partial, racing summary.
	for _, state := range []string{"draft", "lobby", "question_open", "question_closed", "revealed", "leaderboard"} {
		games := finishedGame()
		games.game.State = state

		rec := getResults(t, games)
		if rec.Code != http.StatusConflict {
			t.Errorf("%s: GET /results = %d, want 409 (body %s)", state, rec.Code, rec.Body)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FINISHED" {
			t.Errorf("%s: error code = %q, want GAME_NOT_FINISHED", state, code)
		}
		// The guard must run BEFORE the reads, not alongside them — a guard
		// that races the reads is one that can leak a partial answer.
		if len(games.leaderboardFor) != 0 || len(games.questionStatsFor) != 0 {
			t.Errorf("%s: unfinished game still hit the store (leaderboard %v, stats %v)",
				state, games.leaderboardFor, games.questionStatsFor)
		}
	}
}

func TestGameResultsForeignOrMissingGameReturns404(t *testing.T) {
	// Ownership lives in GetGameForOrganizer's WHERE clause, so a foreign
	// game is indistinguishable from a missing one: 404, never 403.
	games := &stubGames{gameErr: store.ErrNotFound}

	rec := getResults(t, games)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /results on foreign game = %d, want 404 (body %s)", rec.Code, rec.Body)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
	if len(games.leaderboardFor) != 0 || len(games.questionStatsFor) != 0 {
		t.Error("foreign game still reached the post-game reads")
	}
}

func TestGameResultsMalformedGameIDReturns404(t *testing.T) {
	games := finishedGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games/not-a-uuid/results", ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /results with malformed id = %d, want 404 (body %s)", rec.Code, rec.Body)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
	if len(games.gotGame) != 0 || len(games.leaderboardFor) != 0 || len(games.questionStatsFor) != 0 {
		t.Error("malformed id reached the store instead of short-circuiting")
	}
}

func TestGameResultsEmptyGameSerializesEmptyArrays(t *testing.T) {
	// A game whose lobby opened and closed with nobody joined is ordinary.
	// Assert on the RAW body: a decoded nil and [] are indistinguishable in
	// Go, so only the JSON text can catch a null regression.
	games := finishedGame()

	rec := getResults(t, games)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /results on empty game = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"leaderboard":[]`) {
		t.Errorf("body = %s, want \"leaderboard\":[] (empty array, not null)", raw)
	}
	if !strings.Contains(raw, `"questions":[]`) {
		t.Errorf("body = %s, want \"questions\":[] (empty array, not null)", raw)
	}
	if body := decodeResults(t, rec); body.PlayerCount != 0 {
		t.Errorf("playerCount = %d, want 0", body.PlayerCount)
	}
}

func TestGameResultsStoreErrorReturns503(t *testing.T) {
	for name, seed := range map[string]func(*stubGames){
		"leaderboard read fails":    func(s *stubGames) { s.leaderboardErr = errors.New("connection refused") },
		"question stats read fails": func(s *stubGames) { s.questionStatsErr = errors.New("connection refused") },
	} {
		games := finishedGame()
		seed(games)

		rec := getResults(t, games)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: GET /results = %d, want 503 (body %s)", name, rec.Code, rec.Body)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "DB_UNAVAILABLE" {
			t.Errorf("%s: error code = %q, want DB_UNAVAILABLE", name, code)
		}
	}
}

func TestGameResultsQuestionStatsAreOrganizerScoped(t *testing.T) {
	// The per-question query is organizer-scoped in SQL; the handler must
	// forward the session organizer rather than dropping it.
	games := finishedGame()

	if rec := getResults(t, games); rec.Code != http.StatusOK {
		t.Fatalf("GET /results = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if len(games.questionStatsFor) != 1 || games.questionStatsFor[0] != [2]string{testGameID, "org-1"} {
		t.Errorf("question stats queried for %v, want exactly [(%s, org-1)]", games.questionStatsFor, testGameID)
	}
	if len(games.leaderboardFor) != 1 || games.leaderboardFor[0] != testGameID {
		t.Errorf("leaderboard queried for %v, want exactly [%s]", games.leaderboardFor, testGameID)
	}
}
