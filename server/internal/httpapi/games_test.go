package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/auth"
	"github.com/avraham-shor/whatsapp-clickers/internal/store"
	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

const (
	testGameID     = "11111111-2222-3333-4444-555555555555"
	testQuestionID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

// stubGames implements GameStore with canned results and records every
// mutation so tests can assert what reached the store.
type stubGames struct {
	game        gen.Game
	gameErr     error
	rows        []gen.ListGamesByOrganizerRow
	listErr     error
	questions   []gen.Question
	question    gen.Question
	questionErr error
	deleteErr   error
	reorderErr  error
	scoringErr  error
	packages    []gen.ListQuestionPackagesRow
	packagesErr error
	importedQs  []gen.Question
	importErr   error

	leaderboard      []store.ParticipantScore
	leaderboardErr   error
	questionStats    []store.QuestionResponseStats
	questionStatsErr error

	createdTitles    []string
	listedFor        []string
	gotGame          [][2]string
	createdQs        []store.CreateQuestionParams
	updatedQs        []store.UpdateQuestionParams
	deletedQs        [][3]string
	reorders         [][]string
	scoringUpdates   []store.UpdateGameScoringParams
	imports          [][3]string
	leaderboardFor   []string
	questionStatsFor [][2]string
}

func (s *stubGames) CreateGame(ctx context.Context, organizerID, title string) (gen.Game, error) {
	s.createdTitles = append(s.createdTitles, title)
	return s.game, s.gameErr
}

func (s *stubGames) ListGamesByOrganizer(ctx context.Context, organizerID string) ([]gen.ListGamesByOrganizerRow, error) {
	s.listedFor = append(s.listedFor, organizerID)
	return s.rows, s.listErr
}

func (s *stubGames) GetGameForOrganizer(ctx context.Context, gameID, organizerID string) (gen.Game, error) {
	s.gotGame = append(s.gotGame, [2]string{gameID, organizerID})
	return s.game, s.gameErr
}

func (s *stubGames) ListQuestionsByGame(ctx context.Context, gameID, organizerID string) ([]gen.Question, error) {
	return s.questions, s.listErr
}

func (s *stubGames) CreateQuestion(ctx context.Context, arg store.CreateQuestionParams) (gen.Question, error) {
	s.createdQs = append(s.createdQs, arg)
	return s.question, s.questionErr
}

func (s *stubGames) UpdateQuestion(ctx context.Context, arg store.UpdateQuestionParams) (gen.Question, error) {
	s.updatedQs = append(s.updatedQs, arg)
	return s.question, s.questionErr
}

func (s *stubGames) DeleteQuestion(ctx context.Context, questionID, gameID, organizerID string) error {
	s.deletedQs = append(s.deletedQs, [3]string{questionID, gameID, organizerID})
	return s.deleteErr
}

func (s *stubGames) ReorderQuestions(ctx context.Context, gameID, organizerID string, orderedIDs []string) error {
	s.reorders = append(s.reorders, orderedIDs)
	return s.reorderErr
}

func (s *stubGames) UpdateGameScoring(ctx context.Context, arg store.UpdateGameScoringParams) (gen.Game, error) {
	s.scoringUpdates = append(s.scoringUpdates, arg)
	if s.scoringErr != nil {
		return gen.Game{}, s.scoringErr
	}
	// Mirror the DB: return the stored game with the new scoring applied.
	game := s.game
	game.PointsPerCorrect = arg.PointsPerCorrect
	game.SpeedBonusFirst = arg.SpeedBonusFirst
	game.SpeedBonusSecond = arg.SpeedBonusSecond
	game.SpeedBonusThird = arg.SpeedBonusThird
	return game, nil
}

func (s *stubGames) ListQuestionPackages(ctx context.Context) ([]gen.ListQuestionPackagesRow, error) {
	return s.packages, s.packagesErr
}

func (s *stubGames) ImportPackageQuestions(ctx context.Context, gameID, organizerID, packageID string) ([]gen.Question, error) {
	s.imports = append(s.imports, [3]string{gameID, organizerID, packageID})
	return s.importedQs, s.importErr
}

// The two post-game read methods (story 3.10) are called from the request
// goroutine only — this story spawns nothing after the response, unlike
// control.go's dispatch paths — so these need no mutex and no
// copy-under-lock accessor, unlike control_test.go's stubs.
func (s *stubGames) GetLeaderboard(ctx context.Context, gameID string) ([]store.ParticipantScore, error) {
	s.leaderboardFor = append(s.leaderboardFor, gameID)
	return s.leaderboard, s.leaderboardErr
}

func (s *stubGames) ListQuestionResponseStats(ctx context.Context, gameID, organizerID string) ([]store.QuestionResponseStats, error) {
	s.questionStatsFor = append(s.questionStatsFor, [2]string{gameID, organizerID})
	return s.questionStats, s.questionStatsErr
}

// noGames is the GameStore for tests that never touch game routes.
func noGames() *stubGames { return &stubGames{} }

// draftGame returns a stub seeded with an owned draft game, the common
// starting state for mutation tests. Scoring carries the schema defaults.
func draftGame() *stubGames {
	return &stubGames{game: gen.Game{
		ID:               testGameID,
		OrganizerID:      "org-1",
		Title:            "ערב טריוויה",
		JoinCode:         "AB2CD3",
		State:            "draft",
		PointsPerCorrect: 100,
		SpeedBonusFirst:  50,
		SpeedBonusSecond: 30,
		SpeedBonusThird:  20,
		CreatedAt:        time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
	}}
}

// finishedGame returns a stub seeded with an owned finished game — the
// starting state for the post-game results tests (story 3.10).
func finishedGame() *stubGames {
	games := draftGame()
	games.game.State = "finished"
	return games
}

// validScoringBody is a well-formed full-replacement scoring request; tests
// that probe one field start from here.
const validScoringBody = `{"pointsPerCorrect":200,"speedBonusFirst":100,"speedBonusSecond":40,"speedBonusThird":0}`

// gamesRouter builds a router with an authenticated org-1 session.
func gamesRouter(games GameStore) http.Handler {
	svc := &stubAuth{authOrg: auth.Organizer{ID: "org-1", Username: "avraham"}}
	return NewRouter(stubPinger{}, svc, games, testStatic(), nil, nil, nil, nil, nil, nil, nil)
}

// authedRequest carries the session cookie the stubAuth accepts.
func authedRequest(method, target, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	req.AddCookie(&http.Cookie{Name: "wc_session", Value: "raw-token-value"})
	return req
}

func TestCreateGameReturnsJoinCode(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", `{"title":"  ערב טריוויה  "}`))

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/games = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		ID            string          `json:"id"`
		Title         string          `json:"title"`
		JoinCode      string          `json:"joinCode"`
		State         string          `json:"state"`
		QuestionCount int             `json:"questionCount"`
		Questions     json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("create response is not valid JSON: %v", err)
	}
	if body.JoinCode != "AB2CD3" || body.State != "draft" || body.ID != testGameID {
		t.Errorf("create body = %+v, want the stored game with its join code", body)
	}
	if string(body.Questions) != "[]" {
		t.Errorf("questions = %s, want [] (empty array, not null)", body.Questions)
	}
	if len(games.createdTitles) != 1 || games.createdTitles[0] != "ערב טריוויה" {
		t.Errorf("store received titles %v, want the trimmed title", games.createdTitles)
	}
}

func TestCreateGameTitleValidation(t *testing.T) {
	for name, body := range map[string]string{
		"empty":      `{"title":""}`,
		"whitespace": `{"title":"   "}`,
		"too long":   `{"title":"` + strings.Repeat("א", 121) + `"}`,
		"missing":    `{}`,
	} {
		games := draftGame()
		rec := httptest.NewRecorder()
		gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", body))

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: POST /api/games = %d, want 400", name, rec.Code)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "VALIDATION_FAILED" {
			t.Errorf("%s: error code = %q, want VALIDATION_FAILED", name, code)
		}
		if len(games.createdTitles) != 0 {
			t.Errorf("%s: invalid title reached the store", name)
		}
	}
}

func TestCreateGameTitleBoundary120RunesAccepted(t *testing.T) {
	// Hebrew is multi-byte — the limit must count runes, not bytes.
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"title":"` + strings.Repeat("א", 120) + `"}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("120-rune title = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
}

func TestListGamesScopedToSessionOrganizer(t *testing.T) {
	games := &stubGames{rows: []gen.ListGamesByOrganizerRow{{
		ID:            testGameID,
		OrganizerID:   "org-1",
		Title:         "ערב טריוויה",
		JoinCode:      "AB2CD3",
		State:         "draft",
		QuestionCount: 3,
	}}}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/games = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if len(games.listedFor) != 1 || games.listedFor[0] != "org-1" {
		t.Fatalf("list queried for %v, want exactly the session organizer org-1", games.listedFor)
	}
	var body struct {
		Items []struct {
			JoinCode      string `json:"joinCode"`
			QuestionCount int    `json:"questionCount"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("list response is not valid JSON: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].JoinCode != "AB2CD3" || body.Items[0].QuestionCount != 3 {
		t.Errorf("items = %+v, want one game with joinCode AB2CD3 and questionCount 3", body.Items)
	}
}

func TestGetGameIncludesQuestions(t *testing.T) {
	games := draftGame()
	games.questions = []gen.Question{{
		ID:               testQuestionID,
		GameID:           testGameID,
		Position:         1,
		Type:             "mcq",
		Text:             "מה בירת צרפת?",
		Options:          []string{"פריז", "ליון", "מרסיי", "ניס"},
		CorrectOption:    1,
		AcceptedAnswers:  []string{},
		TimeLimitSeconds: 20,
	}}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games/"+testGameID, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/games/{id} = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		JoinCode  string `json:"joinCode"`
		Questions []struct {
			Type            string   `json:"type"`
			CorrectOption   int      `json:"correctOption"`
			Options         []string `json:"options"`
			AcceptedAnswers []string `json:"acceptedAnswers"`
		} `json:"questions"`
		QuestionCount int `json:"questionCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("game detail is not valid JSON: %v", err)
	}
	if body.JoinCode != "AB2CD3" || len(body.Questions) != 1 || body.QuestionCount != 1 {
		t.Fatalf("detail = %+v, want the game with its one question", body)
	}
	q := body.Questions[0]
	if q.Type != "mcq" || q.CorrectOption != 1 || len(q.Options) != 4 {
		t.Errorf("question = %+v, want mcq with 4 options and correctOption 1", q)
	}
	if q.AcceptedAnswers != nil {
		t.Errorf("acceptedAnswers = %v, want omitted for mcq (sentinels never reach the wire)", q.AcceptedAnswers)
	}
}

func TestForeignGameReturns404Envelope(t *testing.T) {
	// Ownership scoping (AC-5): a foreign game is indistinguishable from a
	// missing one — 404 GAME_NOT_FOUND, never 403.
	games := &stubGames{gameErr: store.ErrNotFound}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games/"+testGameID, ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET foreign game = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
}

func TestMalformedGameIDReturns404WithoutStoreCall(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games/not-a-uuid", ""))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/games/not-a-uuid = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
	if len(games.gotGame) != 0 {
		t.Error("malformed id reached the store instead of short-circuiting")
	}
}

func TestGameMutationsWithoutSessionReturn401(t *testing.T) {
	router := NewRouter(stubPinger{}, noAuth(), draftGame(), testStatic(), nil, nil, nil, nil, nil, nil, nil)
	for name, req := range map[string]*http.Request{
		"create game":     httptest.NewRequest(http.MethodPost, "/api/games", strings.NewReader(`{"title":"x"}`)),
		"list games":      httptest.NewRequest(http.MethodGet, "/api/games", nil),
		"get game":        httptest.NewRequest(http.MethodGet, "/api/games/"+testGameID, nil),
		"create question": httptest.NewRequest(http.MethodPost, "/api/games/"+testGameID+"/questions", strings.NewReader(`{}`)),
		"delete question": httptest.NewRequest(http.MethodDelete, "/api/games/"+testGameID+"/questions/"+testQuestionID, nil),
		"reorder":         httptest.NewRequest(http.MethodPost, "/api/games/"+testGameID+"/questions/reorder", strings.NewReader(`{}`)),
		"update scoring":  httptest.NewRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", strings.NewReader(validScoringBody)),
		"game results":    httptest.NewRequest(http.MethodGet, "/api/games/"+testGameID+"/results", nil),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without session = %d, want 401", name, rec.Code)
		}
	}
}

func TestCreateGameBodyTooLargeReturns413(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"title":"` + strings.Repeat("x", 65<<10) + `"}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", body))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized create-game body = %d, want 413", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "REQUEST_TOO_LARGE" {
		t.Errorf("error code = %q, want REQUEST_TOO_LARGE", code)
	}
}

func TestCreateGameStoreFailureReturns503(t *testing.T) {
	games := &stubGames{gameErr: errors.New("connection refused")}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPost, "/api/games", `{"title":"ערב"}`))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("create game with dead DB = %d, want 503", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "DB_UNAVAILABLE" {
		t.Errorf("error code = %q, want DB_UNAVAILABLE", code)
	}
}

func TestUpdateScoringPersistsAndEchoes(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", validScoringBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /scoring = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	var body struct {
		PointsPerCorrect *int `json:"pointsPerCorrect"`
		SpeedBonusFirst  *int `json:"speedBonusFirst"`
		SpeedBonusSecond *int `json:"speedBonusSecond"`
		SpeedBonusThird  *int `json:"speedBonusThird"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("scoring response is not valid JSON: %v", err)
	}
	// speedBonusThird is 0 and must still be present on the wire (a disabled
	// bonus is a value, not an omission) — hence pointer targets.
	if body.PointsPerCorrect == nil || *body.PointsPerCorrect != 200 ||
		body.SpeedBonusFirst == nil || *body.SpeedBonusFirst != 100 ||
		body.SpeedBonusSecond == nil || *body.SpeedBonusSecond != 40 ||
		body.SpeedBonusThird == nil || *body.SpeedBonusThird != 0 {
		t.Errorf("scoring echo = %s, want 200/100/40/0 with all fields present", rec.Body)
	}
	if len(games.scoringUpdates) != 1 {
		t.Fatalf("store received %d scoring updates, want 1", len(games.scoringUpdates))
	}
	got := games.scoringUpdates[0]
	if got.GameID != testGameID || got.OrganizerID != "org-1" ||
		got.PointsPerCorrect != 200 || got.SpeedBonusFirst != 100 ||
		got.SpeedBonusSecond != 40 || got.SpeedBonusThird != 0 {
		t.Errorf("store params = %+v, want the validated request values scoped to org-1", got)
	}
}

func TestUpdateScoringValidation(t *testing.T) {
	for name, body := range map[string]string{
		"negative points":       `{"pointsPerCorrect":-1,"speedBonusFirst":50,"speedBonusSecond":30,"speedBonusThird":20}`,
		"negative bonus":        `{"pointsPerCorrect":100,"speedBonusFirst":50,"speedBonusSecond":-5,"speedBonusThird":20}`,
		"points over cap":       `{"pointsPerCorrect":10001,"speedBonusFirst":50,"speedBonusSecond":30,"speedBonusThird":20}`,
		"bonus over cap":        `{"pointsPerCorrect":100,"speedBonusFirst":50,"speedBonusSecond":30,"speedBonusThird":10001}`,
		"missing points":        `{"speedBonusFirst":50,"speedBonusSecond":30,"speedBonusThird":20}`,
		"missing first bonus":   `{"pointsPerCorrect":100,"speedBonusSecond":30,"speedBonusThird":20}`,
		"missing second bonus":  `{"pointsPerCorrect":100,"speedBonusFirst":50,"speedBonusThird":20}`,
		"missing third bonus":   `{"pointsPerCorrect":100,"speedBonusFirst":50,"speedBonusSecond":30}`,
		"null field (explicit)": `{"pointsPerCorrect":null,"speedBonusFirst":50,"speedBonusSecond":30,"speedBonusThird":20}`,
	} {
		games := draftGame()
		rec := httptest.NewRecorder()
		gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", body))

		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: PUT /scoring = %d, want 400", name, rec.Code)
			continue
		}
		if code := decodeErrorCode(t, rec.Body.Bytes()); code != "VALIDATION_FAILED" {
			t.Errorf("%s: error code = %q, want VALIDATION_FAILED", name, code)
		}
		if len(games.scoringUpdates) != 0 {
			t.Errorf("%s: invalid scoring reached the store", name)
		}
	}
}

func TestUpdateScoringBoundaryValuesAccepted(t *testing.T) {
	// 0 disables a bonus (and is legal for points); 10000 is the cap.
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"pointsPerCorrect":0,"speedBonusFirst":10000,"speedBonusSecond":0,"speedBonusThird":10000}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("boundary scoring values = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
}

func TestUpdateScoringNonIntegerReturns400(t *testing.T) {
	// 1.5 fails the int32 decode — INVALID_REQUEST, not VALIDATION_FAILED.
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"pointsPerCorrect":1.5,"speedBonusFirst":50,"speedBonusSecond":30,"speedBonusThird":20}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-integer scoring = %d, want 400", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "INVALID_REQUEST" {
		t.Errorf("error code = %q, want INVALID_REQUEST", code)
	}
}

func TestUpdateScoringForeignGameReturns404(t *testing.T) {
	games := &stubGames{gameErr: store.ErrNotFound}
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", validScoringBody))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("PUT scoring on foreign game = %d, want 404", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_FOUND" {
		t.Errorf("error code = %q, want GAME_NOT_FOUND", code)
	}
}

func TestUpdateScoringMalformedGameIDReturns404WithoutStoreCall(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/not-a-uuid/scoring", validScoringBody))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("PUT scoring with malformed id = %d, want 404", rec.Code)
	}
	if len(games.gotGame) != 0 || len(games.scoringUpdates) != 0 {
		t.Error("malformed id reached the store instead of short-circuiting")
	}
}

func TestUpdateScoringNonDraftReturns409(t *testing.T) {
	games := draftGame()
	games.game.State = "lobby"
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", validScoringBody))

	if rec.Code != http.StatusConflict {
		t.Fatalf("PUT scoring on non-draft game = %d, want 409", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "GAME_NOT_EDITABLE" {
		t.Errorf("error code = %q, want GAME_NOT_EDITABLE", code)
	}
	if len(games.scoringUpdates) != 0 {
		t.Error("non-draft scoring update reached the store")
	}
}

func TestUpdateScoringDraftCheckPrecedesValidation(t *testing.T) {
	// The game's state answers before the body is judged (handleUpdateQuestion
	// order): a non-draft or foreign game gets its 409/404 even when the body
	// is also invalid — never a misleading VALIDATION_FAILED.
	invalidBody := `{"pointsPerCorrect":-1,"speedBonusFirst":50,"speedBonusSecond":30,"speedBonusThird":20}`

	nonDraft := draftGame()
	nonDraft.game.State = "lobby"
	rec := httptest.NewRecorder()
	gamesRouter(nonDraft).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", invalidBody))
	if rec.Code != http.StatusConflict {
		t.Errorf("invalid body on non-draft game = %d, want 409", rec.Code)
	}

	foreign := &stubGames{gameErr: store.ErrNotFound}
	rec = httptest.NewRecorder()
	gamesRouter(foreign).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", invalidBody))
	if rec.Code != http.StatusNotFound {
		t.Errorf("invalid body on foreign game = %d, want 404", rec.Code)
	}
}

func TestUpdateScoringBodyTooLargeReturns413(t *testing.T) {
	games := draftGame()
	rec := httptest.NewRecorder()
	body := `{"pointsPerCorrect":100,"speedBonusFirst":50,"speedBonusSecond":30,"speedBonusThird":20,"junk":"` + strings.Repeat("x", 65<<10) + `"}`
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodPut, "/api/games/"+testGameID+"/scoring", body))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized scoring body = %d, want 413", rec.Code)
	}
	if code := decodeErrorCode(t, rec.Body.Bytes()); code != "REQUEST_TOO_LARGE" {
		t.Errorf("error code = %q, want REQUEST_TOO_LARGE", code)
	}
}

func TestGamePayloadsCarryScoring(t *testing.T) {
	// AC-1: defaults live in the schema and flow through every game payload —
	// detail (editor pre-fill) and list alike.
	games := draftGame()
	rec := httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games/"+testGameID, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/games/{id} = %d, want 200", rec.Code)
	}
	var detail struct {
		PointsPerCorrect int `json:"pointsPerCorrect"`
		SpeedBonusFirst  int `json:"speedBonusFirst"`
		SpeedBonusSecond int `json:"speedBonusSecond"`
		SpeedBonusThird  int `json:"speedBonusThird"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("game detail is not valid JSON: %v", err)
	}
	if detail.PointsPerCorrect != 100 || detail.SpeedBonusFirst != 50 ||
		detail.SpeedBonusSecond != 30 || detail.SpeedBonusThird != 20 {
		t.Errorf("detail scoring = %+v, want the 100/50/30/20 defaults", detail)
	}

	// The list handler copies gen.Game field-by-field — regression-guard the
	// scoring fields there too.
	games.rows = []gen.ListGamesByOrganizerRow{{
		ID:               testGameID,
		OrganizerID:      "org-1",
		Title:            "ערב טריוויה",
		JoinCode:         "AB2CD3",
		State:            "draft",
		PointsPerCorrect: 100,
		SpeedBonusFirst:  50,
		SpeedBonusSecond: 30,
		SpeedBonusThird:  20,
	}}
	rec = httptest.NewRecorder()
	gamesRouter(games).ServeHTTP(rec, authedRequest(http.MethodGet, "/api/games", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/games = %d, want 200", rec.Code)
	}
	var list struct {
		Items []struct {
			PointsPerCorrect int `json:"pointsPerCorrect"`
			SpeedBonusFirst  int `json:"speedBonusFirst"`
			SpeedBonusSecond int `json:"speedBonusSecond"`
			SpeedBonusThird  int `json:"speedBonusThird"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list response is not valid JSON: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].PointsPerCorrect != 100 ||
		list.Items[0].SpeedBonusFirst != 50 || list.Items[0].SpeedBonusSecond != 30 ||
		list.Items[0].SpeedBonusThird != 20 {
		t.Errorf("list scoring = %+v, want 100/50/30/20 on the list payload", list.Items)
	}
}
